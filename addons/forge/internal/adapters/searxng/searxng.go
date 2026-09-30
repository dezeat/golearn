// Copyright 2026 dezeat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package searxng

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

// searchPath is SearXNG's JSON API route, joined onto the configured endpoint.
// The endpoint is the instance root so that a reverse-proxied deployment under
// a subpath keeps its prefix.
const searchPath = "search"

// citationKeyPrefix and citationKeyHexLen shape the evidence id. The key is
// derived from the URL rather than from the result's position, so that two
// planned queries surfacing the same page produce the same citation key and a
// citation can be resolved back to its record. It is truncated because the key
// is repeated in every prompt fence: 48 bits is far more than enough to keep a
// handful of sources per run distinct, and a full digest would be noise the
// model has to carry.
const (
	citationKeyPrefix = "src-"
	citationKeyHexLen = 12
)

// Config is the adapter's runtime configuration.
//
// Every bound is required and none is silently defaulted. A default here would
// be a policy the composition root never saw and could not override, which is
// exactly the kind of invisible choice FORGE.md 5 keeps out of adapters. The
// endpoint in particular is supplied at runtime and never compiled in.
type Config struct {
	// BaseURL is the SearXNG instance root, e.g. https://searxng.example.test
	// or http://host:8080/searxng/. Supplied by the operator at runtime.
	BaseURL string
	// HTTPClient is optional. When nil the adapter uses a private client with
	// no client-level timeout, because ports.Query.Timeout and the caller's
	// context are the budget. Sharing http.DefaultClient is deliberately not
	// the fallback: its transport is process-global, so one component's
	// settings would silently apply to every other.
	HTTPClient *http.Client
	// MaxResponseBytes bounds the response body the adapter will read. It is
	// separate from ports.Query.MaxBytesPerSource, which bounds the content of
	// one evidence record: a single malicious or misconfigured response can be
	// far larger than any snippet it claims to carry.
	MaxResponseBytes int64
	// MaxAttempts is the total number of requests one Gather may make,
	// including the first. FORGE.md 5 requires bounded retry before failing
	// clear, and unbounded retry is how a "clear failure" becomes a hang.
	MaxAttempts int
	// RetryBackoff is the constant delay between attempts. Zero means no delay
	// and is valid. It is constant rather than jittered because a seeded PRNG
	// is the project's only sanctioned randomness and none of it belongs in a
	// transport.
	RetryBackoff time.Duration
	// Now supplies the retrieval timestamp. domain.Evidence documents that
	// RetrievedAt is injected rather than read from the clock inside a
	// constructor, so fixture-backed tests stay deterministic. Nil means
	// time.Now.
	Now func() time.Time
}

// Adapter implements ports.Research against a SearXNG instance.
type Adapter struct {
	endpoint     *url.URL
	client       *http.Client
	maxBytes     int64
	maxAttempts  int
	retryBackoff time.Duration
	now          func() time.Time
}

// The port binding is a compile-time fact, not something discovered at wiring
// time in the composition root.
var _ ports.Research = (*Adapter)(nil)

// New validates the configuration and binds the adapter to an endpoint.
func New(cfg Config) (*Adapter, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("searxng: no endpoint configured")
	}
	endpoint, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("searxng: endpoint is not a URL: %w", err)
	}
	// The scheme allowlist is a foot-gun guard, not a security boundary: a
	// mistyped endpoint should fail at construction rather than turn into a
	// file read at request time. Plain http stays allowed because a
	// self-hosted instance on a private network commonly speaks it.
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, fmt.Errorf("searxng: endpoint scheme %q is not http or https", endpoint.Scheme)
	}
	if endpoint.Host == "" {
		return nil, fmt.Errorf("searxng: endpoint has no host")
	}
	if cfg.MaxResponseBytes <= 0 {
		return nil, fmt.Errorf("searxng: MaxResponseBytes must be positive, got %d", cfg.MaxResponseBytes)
	}
	if cfg.MaxAttempts <= 0 {
		return nil, fmt.Errorf("searxng: MaxAttempts must be at least 1, got %d", cfg.MaxAttempts)
	}
	if cfg.RetryBackoff < 0 {
		return nil, fmt.Errorf("searxng: RetryBackoff must not be negative, got %s", cfg.RetryBackoff)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return &Adapter{
		endpoint:     endpoint,
		client:       client,
		maxBytes:     cfg.MaxResponseBytes,
		maxAttempts:  cfg.MaxAttempts,
		retryBackoff: cfg.RetryBackoff,
		now:          now,
	}, nil
}

// Gather executes one bounded query and returns the evidence it yielded.
//
// Zero records with a nil error is a legitimate outcome: "nothing found" and
// "the search failed" are different facts, and ports.Research reserves the
// judgement about which is fatal for the pipeline.
func (a *Adapter) Gather(ctx context.Context, q ports.Query) ([]domain.Evidence, error) {
	if err := validateQuery(q); err != nil {
		return nil, err
	}

	// A zero Timeout means "no adapter budget"; the caller's context still
	// governs. Deriving a context unconditionally would turn an unset budget
	// into an already-expired deadline.
	if q.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, q.Timeout)
		defer cancel()
	}

	body, err := a.fetch(ctx, q)
	if err != nil {
		return nil, err
	}

	var parsed searchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("searxng: %w: response did not parse as JSON: %w",
			domain.ErrResearchResponse, err)
	}

	return a.toEvidence(parsed.Results, q), nil
}

func validateQuery(q ports.Query) error {
	if strings.TrimSpace(q.Terms) == "" {
		return fmt.Errorf("searxng: query has no terms")
	}
	if q.MaxResults <= 0 {
		return fmt.Errorf("searxng: query MaxResults must be positive, got %d", q.MaxResults)
	}
	if q.MaxBytesPerSource <= 0 {
		return fmt.Errorf("searxng: query MaxBytesPerSource must be positive, got %d", q.MaxBytesPerSource)
	}
	if q.Timeout < 0 {
		return fmt.Errorf("searxng: query Timeout must not be negative, got %s", q.Timeout)
	}
	return nil
}

// fetch performs the bounded, retrying request and returns the raw body.
//
// The retry loop distinguishes three exits deliberately. A context error ends
// it immediately — the caller stopped us, and neither an attempt ceiling nor a
// provider fault describes that. A non-retryable provider answer ends it after
// one request, so a misconfiguration is reported rather than hammered. Only a
// transient failure consumes the budget.
func (a *Adapter) fetch(ctx context.Context, q ports.Query) ([]byte, error) {
	requestURL := a.requestURL(q)

	var lastErr error
	for attempt := 1; attempt <= a.maxAttempts; attempt++ {
		if attempt > 1 {
			if err := a.waitBeforeRetry(ctx); err != nil {
				return nil, err
			}
		}

		body, retryable, err := a.attempt(ctx, requestURL)
		switch {
		case err == nil:
			return body, nil
		case ctx.Err() != nil:
			// Checked before the retryable branch: a canceled or expired
			// context surfaces as a transport failure, and reporting it as an
			// unreachable provider would blame the wrong party.
			return nil, fmt.Errorf("searxng: research call stopped: %w", ctx.Err())
		case !retryable:
			return nil, err
		}
		lastErr = err
	}

	// Two %w verbs so errors.Is answers both "did the budget run out" and
	// "what kept failing". Reporting only the budget would leave an operator
	// with no idea what to fix.
	return nil, fmt.Errorf("searxng: %w after %d attempts: %w",
		domain.ErrBudgetExhausted, a.maxAttempts, lastErr)
}

// attempt makes one request. The bool reports whether the failure is worth
// retrying.
func (a *Adapter) attempt(ctx context.Context, requestURL string) (body []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("searxng: building the request failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		// An unanswered request. Transient by assumption — a restarting
		// instance and a wrong hostname are indistinguishable from here, and
		// the attempt ceiling bounds the cost of being wrong.
		return nil, true, fmt.Errorf("searxng: %w: %w", domain.ErrProviderUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, retryableStatus(resp.StatusCode), a.statusError(resp.StatusCode)
	}

	// Read one byte past the bound so the overflow is detectable. io.LimitReader
	// signals truncation by stopping, not by returning an error, so an adapter
	// that trusted the read error would happily parse half a document as if it
	// were whole. Measured in docs/design/FORGE-EXPERIMENTS.md A-13.
	body, err = io.ReadAll(io.LimitReader(resp.Body, a.maxBytes+1))
	if err != nil {
		return nil, true, fmt.Errorf("searxng: %w: reading the response failed: %w",
			domain.ErrProviderUnreachable, err)
	}
	if int64(len(body)) > a.maxBytes {
		return nil, false, fmt.Errorf("searxng: %w: response exceeds the %d-byte bound",
			domain.ErrResearchResponse, a.maxBytes)
	}
	return body, false, nil
}

// waitBeforeRetry sleeps, but stays cancellable. A plain time.Sleep would let a
// backoff outlive the run that scheduled it.
func (a *Adapter) waitBeforeRetry(ctx context.Context) error {
	if a.retryBackoff == 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(a.retryBackoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("searxng: research call stopped: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// retryableStatus reports whether a status is worth another request. A refusal
// or a bad request will be refused identically next time; only overload and
// server faults are worth waiting out.
func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func (a *Adapter) statusError(status int) error {
	if status == http.StatusForbidden {
		// Named because it is the setup mistake nearly every SearXNG operator
		// makes once: the JSON API answers 403 until the format is enabled.
		return fmt.Errorf("searxng: %w: status 403 — the instance may not have "+
			"json listed under search.formats in settings.yml",
			domain.ErrResearchResponse)
	}
	return fmt.Errorf("searxng: %w: status %d", domain.ErrResearchResponse, status)
}

// requestURL builds the bounded search request. Only the parameters the
// pipeline expressed are sent: an adapter that added categories, engines or a
// safe-search level would be making source policy, which is #120's to make.
func (a *Adapter) requestURL(q ports.Query) string {
	u := a.endpoint.JoinPath(searchPath)
	params := url.Values{}
	params.Set("q", q.Terms)
	params.Set("format", "json")
	if q.Language != "" {
		params.Set("language", q.Language)
	}
	u.RawQuery = params.Encode()
	return u.String()
}

// searchResponse and searchResult decode only the fields this adapter uses.
//
// Decoding is deliberately lenient — no DisallowUnknownFields — because
// SearXNG's result struct carries a dozen more fields (engine, engines, score,
// category, positions, parsed_url, publishedDate, template, thumbnail and
// others) and grows across releases. Strict decoding would turn every upstream
// field addition into an outage, and requiring a field the adapter does not
// use would be a dependency on someone else's schema for no benefit.
type searchResponse struct {
	Results []searchResult `json:"results"`
}

type searchResult struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// toEvidence maps results onto the standardized record.
//
// Publisher is left empty on purpose: SearXNG exposes no publisher field, and
// domain.Evidence requires that title and publisher are recorded where the
// source exposes them and left empty rather than guessed where it does not. A
// hostname is a guess.
//
// Results are not de-duplicated. SearXNG already merges the same URL across
// engines upstream — the engines and positions fields on its result type are
// the evidence of that merge — so a second de-duplication here would be
// inventing URL-canonicalization policy on top of work already done.
func (a *Adapter) toEvidence(results []searchResult, q ports.Query) []domain.Evidence {
	retrievedAt := a.now()
	evidence := make([]domain.Evidence, 0, len(results))

	for _, r := range results {
		if len(evidence) >= q.MaxResults {
			break
		}
		// A record with no URL cannot be cited, and Evidence.Ref carries the
		// URL into the shipped pack, so it has no downstream use.
		if strings.TrimSpace(r.URL) == "" {
			continue
		}
		evidence = append(evidence, domain.Evidence{
			ID:          citationKey(r.URL),
			URL:         r.URL,
			Title:       r.Title,
			RetrievedAt: retrievedAt,
			Content:     domain.Untrusted(truncateAtRuneBoundary(r.Content, q.MaxBytesPerSource)),
			// Quality is deliberately left at its zero value: the
			// source-authority policy (#120) makes that judgement, not this
			// adapter.
		})
	}
	return evidence
}

// citationKey derives a stable evidence id from the URL exactly as returned.
// The URL is not normalized first; canonicalization is a policy decision, and
// guessing at one here would silently merge or split sources behind the
// pipeline's back.
func citationKey(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return citationKeyPrefix + hex.EncodeToString(sum[:])[:citationKeyHexLen]
}

// truncateAtRuneBoundary cuts to at most maxBytes without splitting a rune.
// A byte-exact cut would hand the generator invalid UTF-8, and no marker is
// appended because the budget is the pipeline's accounting, not a message to
// the model.
func truncateAtRuneBoundary(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
