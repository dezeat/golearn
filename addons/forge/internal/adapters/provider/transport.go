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

// Package provider implements the four V1 provider profiles behind
// ports.Provider (FORGE.md 6.1).
//
// # Why raw HTTP rather than vendor SDKs
//
// Every provider here ships an official Go SDK, and reaching for them would be
// the usual advice. Two repo rules override it. AGENTS.md fixes a minimal
// dependency footprint and requires explicit authorization for any new
// dependency; four vendor SDKs plus their transitive graphs is the largest
// dependency decision in the project, taken for wire formats that are a few
// dozen lines of JSON each. And FORGE.md 4 asks for one uniform port across
// four providers, which four heterogeneous SDKs with four retry policies,
// four error taxonomies and four notions of a request would work against
// rather than toward. Forge owns stop conditions, budgets and cancellation,
// and it can only own them if nothing underneath is making those decisions.
//
// The cost is real and worth stating: wire-format changes are ours to track.
// It is bounded because Forge uses a deliberately small slice of each API —
// one chat call with a schema, and one embeddings call where the provider has
// them.
//
// # Capability, not configuration
//
// Anthropic ships no embeddings endpoint, so [Anthropic] does not implement
// ports.Embedder at all (D-018). That is a compile-time fact about the vendor,
// not a runtime branch about a key.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
)

// Transport bounds are Forge's, not the provider's. An adapter that retried on
// its own would spend a budget the pipeline is accountable for and make the
// run's cost summary a lie (FORGE.md 4).
const (
	// defaultTimeout bounds one request. Local inference on modest hardware is
	// slow, so this is generous; the pipeline passes a tighter context when it
	// wants one.
	defaultTimeout = 5 * time.Minute

	// maxErrorBodyBytes caps how much of a provider's error body is read for
	// diagnostics. An HTML error page from a proxy is not a useful message and
	// must not become an unbounded read.
	maxErrorBodyBytes = 8 << 10
)

// transport carries the shared HTTP plumbing.
type transport struct {
	client  *http.Client
	baseURL string
	// header populates per-request auth and version headers. Keeping it a
	// function rather than a map means a credential is read at call time and
	// never parked in a struct field that something could format.
	header func(h http.Header)
}

func newTransport(baseURL string, client *http.Client, header func(http.Header)) *transport {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &transport{
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
		header:  header,
	}
}

// postJSON sends body to path and decodes the reply into out.
//
// Errors are classified into the domain taxonomy here, once, so that four
// adapters cannot drift into four different answers for "the service is not
// running".
func (t *transport) postJSON(ctx context.Context, path string, body, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	return t.do(ctx, http.MethodPost, path, bytes.NewReader(encoded), out)
}

// getJSON is the probe path: reachability and model discovery.
func (t *transport) getJSON(ctx context.Context, path string, out any) error {
	return t.do(ctx, http.MethodGet, path, nil, out)
}

func (t *transport) do(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, t.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if t.header != nil {
		t.header(req.Header)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		// Cancellation and deadline are the caller's own signals traveling
		// back; reporting them as "unreachable" would blame the provider for
		// a cancel the user pressed.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("%w: %v", domain.ErrProviderUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return classifyStatus(resp)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%w: reply was not valid JSON: %v", domain.ErrStructuredOutput, err)
	}
	return nil
}

// classifyStatus maps an HTTP status onto the Forge error taxonomy.
//
// The distinctions that matter to a user are the ones preserved: a missing key
// and a rejected key send them to different places, and a rate limit is worth
// waiting on rather than reconfiguring.
func classifyStatus(resp *http.Response) error {
	snippet := readErrorSnippet(resp.Body)

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w (HTTP %d)%s", domain.ErrCredentialRejected, resp.StatusCode, snippet)
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%w (HTTP %d)%s", domain.ErrRateLimited, resp.StatusCode, snippet)
	case resp.StatusCode == http.StatusNotFound:
		// A 404 from a chat endpoint is nearly always an unknown model rather
		// than a wrong path, because the path is ours and the model is theirs.
		return fmt.Errorf("%w (HTTP 404)%s", domain.ErrModelNotAvailable, snippet)
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: provider returned HTTP %d%s", domain.ErrProviderUnreachable, resp.StatusCode, snippet)
	default:
		return fmt.Errorf("provider returned HTTP %d%s", resp.StatusCode, snippet)
	}
}

// readErrorSnippet returns a bounded, single-line fragment of an error body.
//
// It is bounded because an HTML error page from a reverse proxy is not a
// message, and single-line because this text lands in diagnostics that are
// meant to stay concise (FORGE.md 8).
func readErrorSnippet(body io.Reader) string {
	raw, err := io.ReadAll(io.LimitReader(body, maxErrorBodyBytes))
	if err != nil || len(raw) == 0 {
		return ""
	}
	// Redact before anything else touches it. A provider — or a proxy in
	// front of one — that reflects the submitted key back in an error body
	// would otherwise put the credential into a diagnostic, which is the
	// exact text users paste into bug reports.
	text := domain.Redact(strings.Join(strings.Fields(string(raw)), " "))
	const maxSnippet = 300
	if len(text) > maxSnippet {
		text = text[:maxSnippet] + "…"
	}
	return ": " + text
}

// decodeStructured parses a model's textual reply into out.
//
// Models wrap JSON in prose or fences often enough that a bare Unmarshal would
// reject replies that are actually usable, so one narrowing pass is applied
// before giving up. This is not a repair: the single permitted repair is the
// pipeline's to spend (FORGE.md 4), and this changes no content — it only
// finds where the JSON starts and ends.
func decodeStructured(text string, out any) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return fmt.Errorf("%w: the reply was empty", domain.ErrStructuredOutput)
	}
	if err := json.Unmarshal([]byte(trimmed), out); err == nil {
		return nil
	}

	candidate := stripCodeFence(trimmed)
	if err := json.Unmarshal([]byte(candidate), out); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrStructuredOutput, err)
	}
	return nil
}

// stripCodeFence returns the span between the first opening brace or bracket
// and its matching last closing one, which removes both markdown fences and
// leading commentary without interpreting either.
func stripCodeFence(s string) string {
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return s
	}
	end := strings.LastIndexAny(s, "}]")
	if end <= start {
		return s
	}
	return s[start : end+1]
}

// errNoEmbeddings is returned by the helper that guards embedding calls on a
// profile that has none. Kept unexported: callers match on the domain sentinel.
var errNoEmbeddings = domain.ErrNoEmbeddingCapability

func embeddingUnsupported(p domain.Profile) error {
	return fmt.Errorf("%w: %s exposes no embeddings API", errNoEmbeddings, p.DisplayName)
}

var _ = errors.Is
