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

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

// anthropicVersion is the API version header the Messages API requires on
// every request. It is a dated contract, not a client version: pinning it is
// what stops a server-side change from altering the wire format underneath us.
const anthropicVersion = "2023-06-01"

// Anthropic implements the Anthropic Messages API profile.
//
// It deliberately does NOT implement ports.Embedder. Anthropic ships no
// embeddings endpoint, so there is no configuration under which this type
// could return a vector, and D-018 models that as a compile-time absence
// rather than a method that fails at runtime. TestAnthropicDoesNotSatisfyEmbedder
// fails the moment someone adds a stub that pretends otherwise.
type Anthropic struct {
	model string
	tr    *transport
}

var (
	_ ports.Provider      = (*Anthropic)(nil)
	_ ports.ProviderProbe = (*Anthropic)(nil)
)

// NewAnthropic builds an Anthropic client.
func NewAnthropic(profile domain.Profile, model string, secret domain.Secret, opts ...ClientOption) *Anthropic {
	cfg := applyOptions(profile, opts)
	key := secret.Reveal()
	tr := newTransport(cfg.endpoint, cfg.httpClient, func(h http.Header) {
		// The Messages API authenticates with x-api-key, not a bearer token.
		h.Set("x-api-key", key)
		h.Set("anthropic-version", anthropicVersion)
	})
	return &Anthropic{model: model, tr: tr}
}

// Identity returns the provider and model, never the endpoint.
func (a *Anthropic) Identity() domain.ModelIdentity {
	return domain.ModelIdentity{Provider: string(domain.ProfileAnthropic), Model: a.model}
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model        string             `json:"model"`
	MaxTokens    int                `json:"max_tokens"`
	System       string             `json:"system,omitempty"`
	Messages     []anthropicMessage `json:"messages"`
	Temperature  *float64           `json:"temperature,omitempty"`
	OutputConfig *anthropicOutput   `json:"output_config,omitempty"`
}

type anthropicOutput struct {
	Format *anthropicFormat `json:"format,omitempty"`
}

type anthropicFormat struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema,omitempty"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
}

// defaultAnthropicMaxTokens is used when the caller sets no bound. The
// Messages API requires max_tokens on every request, so unlike the other
// profiles there is no "provider default" to fall back to.
const defaultAnthropicMaxTokens = 8192

// Generate performs one Messages request and decodes the structured reply.
func (a *Anthropic) Generate(ctx context.Context, req ports.Request, out any) error {
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = defaultAnthropicMaxTokens
	}

	messages := make([]anthropicMessage, 0, 2)
	// Evidence is a user turn, never folded into System. System is
	// operator-authored instruction; mixing retrieved text into it is exactly
	// the confusion the trust boundary exists to prevent.
	if fenced := renderEvidence(req.Evidence); fenced != "" {
		messages = append(messages, anthropicMessage{Role: "user", Content: fenced})
	}
	messages = append(messages, anthropicMessage{Role: "user", Content: req.User})

	body := anthropicRequest{
		Model:       a.model,
		MaxTokens:   maxTokens,
		System:      req.System,
		Messages:    messages,
		Temperature: req.Temperature,
	}
	if len(req.Schema) > 0 {
		body.OutputConfig = &anthropicOutput{
			Format: &anthropicFormat{Type: "json_schema", Schema: json.RawMessage(req.Schema)},
		}
	}

	var reply anthropicResponse
	if err := a.tr.postJSON(ctx, "/messages", body, &reply); err != nil {
		return fmt.Errorf("anthropic generate: %w", err)
	}

	// A refusal is a successful HTTP call that produced no usable content.
	// Reporting it as a parse failure would send a reader hunting for a schema
	// bug that does not exist.
	if reply.StopReason == "refusal" {
		return fmt.Errorf("anthropic generate: the model declined the request")
	}

	var text string
	for _, block := range reply.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	if out == nil {
		return nil
	}
	if err := decodeStructured(text, out); err != nil {
		return fmt.Errorf("anthropic generate: %w", err)
	}
	return nil
}

// Probe reports reachability and credential acceptance.
func (a *Anthropic) Probe(ctx context.Context) (ports.ProbeResult, error) {
	var reply struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := a.tr.getJSON(ctx, "/models", &reply); err != nil {
		profile, _ := domain.ProfileByID(domain.ProfileAnthropic)
		return classifyProbeError(err, profile)
	}
	models := make([]string, 0, len(reply.Data))
	for _, m := range reply.Data {
		models = append(models, m.ID)
	}
	return ports.ProbeResult{
		Reachable: true, Authenticated: true, Models: models,
		Detail: "Anthropic reachable, credential accepted (no embeddings capability)",
	}, nil
}
