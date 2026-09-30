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

// OpenAICompatible serves both the OpenAI and OpenRouter profiles.
//
// OpenRouter is deliberately not a separate type. It speaks the same wire
// format, and a second near-identical implementation would be two places for
// one behavior to drift — the difference is a base URL and one extra header,
// which is configuration, not a different provider.
type OpenAICompatible struct {
	profile   domain.Profile
	model     string
	embedding string
	tr        *transport
}

// Compile-time proof of the contract, and of the capability: these profiles
// expose embeddings, so they satisfy Embedder as well as Provider.
var (
	_ ports.Provider      = (*OpenAICompatible)(nil)
	_ ports.Embedder      = (*OpenAICompatible)(nil)
	_ ports.ProviderProbe = (*OpenAICompatible)(nil)
)

// NewOpenAICompatible builds an OpenAI or OpenRouter client.
func NewOpenAICompatible(profile domain.Profile, model string, secret domain.Secret, opts ...ClientOption) *OpenAICompatible {
	cfg := applyOptions(profile, opts)
	key := secret.Reveal()
	tr := newTransport(cfg.endpoint, cfg.httpClient, func(h http.Header) {
		h.Set("Authorization", "Bearer "+key)
		if profile.ID == domain.ProfileOpenRouter {
			// OpenRouter asks callers to identify the application. It is
			// public project metadata, never user or machine identity.
			h.Set("X-Title", "golearn-forge")
		}
	})
	return &OpenAICompatible{
		profile:   profile,
		model:     model,
		embedding: cfg.embeddingModel,
		tr:        tr,
	}
}

// Identity returns the provider and model, never the endpoint.
func (c *OpenAICompatible) Identity() domain.ModelIdentity {
	return domain.ModelIdentity{Provider: string(c.profile.ID), Model: c.model}
}

// EmbeddingIdentity names the embedding model, which is not the chat model.
func (c *OpenAICompatible) EmbeddingIdentity() domain.ModelIdentity {
	return domain.ModelIdentity{Provider: string(c.profile.ID), Model: c.embedding}
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model          string          `json:"model"`
	Messages       []openAIMessage `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	ResponseFormat *openAIFormat   `json:"response_format,omitempty"`
}

type openAIFormat struct {
	Type       string           `json:"type"`
	JSONSchema *openAIRawSchema `json:"json_schema,omitempty"`
}

type openAIRawSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

// Generate performs one chat request and decodes the structured reply.
func (c *OpenAICompatible) Generate(ctx context.Context, req ports.Request, out any) error {
	body := openAIRequest{
		Model:       c.model,
		Messages:    openAIMessages(req),
		MaxTokens:   req.MaxOutputTokens,
		Temperature: req.Temperature,
	}
	if len(req.Schema) > 0 {
		body.ResponseFormat = &openAIFormat{
			Type: "json_schema",
			JSONSchema: &openAIRawSchema{
				Name: "forge_response", Strict: true, Schema: json.RawMessage(req.Schema),
			},
		}
	}

	var reply openAIResponse
	if err := c.tr.postJSON(ctx, "/chat/completions", body, &reply); err != nil {
		return fmt.Errorf("%s generate: %w", c.profile.DisplayName, err)
	}
	if len(reply.Choices) == 0 {
		return fmt.Errorf("%s generate: %w: the reply contained no choices",
			c.profile.DisplayName, domain.ErrStructuredOutput)
	}
	if out == nil {
		return nil
	}
	if err := decodeStructured(reply.Choices[0].Message.Content, out); err != nil {
		return fmt.Errorf("%s generate: %w", c.profile.DisplayName, err)
	}
	return nil
}

// openAIMessages renders the request, keeping evidence strictly separate from
// instruction. Retrieved material goes into a user turn as fenced quoted data,
// never concatenated into the system prompt — that separation is the
// prompt-injection boundary FORGE.md 4 requires, and it is why Request carries
// evidence as its own field rather than pre-rendered text.
func openAIMessages(req ports.Request) []openAIMessage {
	messages := make([]openAIMessage, 0, 3)
	if req.System != "" {
		messages = append(messages, openAIMessage{Role: "system", Content: req.System})
	}
	if fenced := renderEvidence(req.Evidence); fenced != "" {
		messages = append(messages, openAIMessage{Role: "user", Content: fenced})
	}
	messages = append(messages, openAIMessage{Role: "user", Content: req.User})
	return messages
}

type openAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed returns one vector per input text, in the same order.
//
// The provider returns an index per item, and it is honored rather than
// assumed: a reordered reply silently mismatching vectors to questions would
// corrupt every similarity verdict downstream and look like a threshold
// problem.
func (c *OpenAICompatible) Embed(ctx context.Context, texts []string) ([]domain.Vector, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if c.embedding == "" {
		return nil, fmt.Errorf("%s embed: no embedding model configured", c.profile.DisplayName)
	}

	var reply openAIEmbeddingResponse
	err := c.tr.postJSON(ctx, "/embeddings",
		openAIEmbeddingRequest{Model: c.embedding, Input: texts}, &reply)
	if err != nil {
		return nil, fmt.Errorf("%s embed: %w", c.profile.DisplayName, err)
	}
	if len(reply.Data) != len(texts) {
		return nil, fmt.Errorf("%s embed: asked for %d vectors, received %d",
			c.profile.DisplayName, len(texts), len(reply.Data))
	}

	vectors := make([]domain.Vector, len(texts))
	for _, item := range reply.Data {
		if item.Index < 0 || item.Index >= len(vectors) {
			return nil, fmt.Errorf("%s embed: reply index %d is outside the request",
				c.profile.DisplayName, item.Index)
		}
		vectors[item.Index] = domain.Vector(item.Embedding)
	}
	for i, v := range vectors {
		if v == nil {
			return nil, fmt.Errorf("%s embed: no vector returned for input %d", c.profile.DisplayName, i)
		}
	}
	return vectors, nil
}

// Probe reports whether the profile is usable without spending a generation.
func (c *OpenAICompatible) Probe(ctx context.Context) (ports.ProbeResult, error) {
	var reply struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.tr.getJSON(ctx, "/models", &reply); err != nil {
		return classifyProbeError(err, c.profile)
	}
	models := make([]string, 0, len(reply.Data))
	for _, m := range reply.Data {
		models = append(models, m.ID)
	}
	return ports.ProbeResult{
		Reachable: true, Authenticated: true, Models: models,
		Detail: fmt.Sprintf("%s reachable, credential accepted", c.profile.DisplayName),
	}, nil
}
