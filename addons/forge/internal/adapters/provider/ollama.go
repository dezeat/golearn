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

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

// Ollama implements the local-inference profile.
//
// It is the only profile with no credential at all. Its "login" is a
// reachability check (FORGE.md 6.1): GET /api/tags answers "is Ollama
// running?" and "which models are installed?" in one call, and a refused
// connection surfaces as "Ollama is not running", never as an auth error.
type Ollama struct {
	model     string
	embedding string
	endpoint  string
	tr        *transport
}

var (
	_ ports.Provider      = (*Ollama)(nil)
	_ ports.Embedder      = (*Ollama)(nil)
	_ ports.ProviderProbe = (*Ollama)(nil)
)

// NewOllama builds a client for a local Ollama endpoint. No credential is
// taken, because none exists to take.
func NewOllama(profile domain.Profile, model string, opts ...ClientOption) *Ollama {
	cfg := applyOptions(profile, opts)
	return &Ollama{
		model:     model,
		embedding: cfg.embeddingModel,
		endpoint:  cfg.endpoint,
		tr:        newTransport(cfg.endpoint, cfg.httpClient, nil),
	}
}

// Identity returns the provider and model. The endpoint is deliberately absent
// — a local deployment address is operator information and this value reaches
// pack provenance, which is a file people share.
func (o *Ollama) Identity() domain.ModelIdentity {
	return domain.ModelIdentity{Provider: string(domain.ProfileOllama), Model: o.model}
}

// EmbeddingIdentity names the embedding model, which on Ollama is always a
// different model from the chat one.
func (o *Ollama) EmbeddingIdentity() domain.ModelIdentity {
	return domain.ModelIdentity{Provider: string(domain.ProfileOllama), Model: o.embedding}
}

type ollamaChatRequest struct {
	Model    string             `json:"model"`
	Messages []ollamaMessage    `json:"messages"`
	Stream   bool               `json:"stream"`
	Format   json.RawMessage    `json:"format,omitempty"`
	Options  *ollamaChatOptions `json:"options,omitempty"`
}

type ollamaChatOptions struct {
	Temperature *float64 `json:"temperature,omitempty"`
	NumPredict  int      `json:"num_predict,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

// Generate performs one chat request.
//
// Streaming is disabled: Forge needs one complete structured document, and a
// streamed reply would have to be reassembled before it could be validated
// anyway. Ollama takes a JSON Schema directly in `format`, which constrains
// decoding server-side rather than asking the model to behave.
func (o *Ollama) Generate(ctx context.Context, req ports.Request, out any) error {
	messages := make([]ollamaMessage, 0, 3)
	if req.System != "" {
		messages = append(messages, ollamaMessage{Role: "system", Content: req.System})
	}
	if fenced := renderEvidence(req.Evidence); fenced != "" {
		messages = append(messages, ollamaMessage{Role: "user", Content: fenced})
	}
	messages = append(messages, ollamaMessage{Role: "user", Content: req.User})

	body := ollamaChatRequest{Model: o.model, Messages: messages, Stream: false}
	if len(req.Schema) > 0 {
		body.Format = json.RawMessage(req.Schema)
	}
	if req.Temperature != nil || req.MaxOutputTokens > 0 {
		body.Options = &ollamaChatOptions{Temperature: req.Temperature, NumPredict: req.MaxOutputTokens}
	}

	var reply ollamaChatResponse
	if err := o.tr.postJSON(ctx, "/api/chat", body, &reply); err != nil {
		return fmt.Errorf("ollama generate: %w", err)
	}
	if out == nil {
		return nil
	}
	if err := decodeStructured(reply.Message.Content, out); err != nil {
		return fmt.Errorf("ollama generate: %w", err)
	}
	return nil
}

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// Embed returns one vector per input text, in order.
func (o *Ollama) Embed(ctx context.Context, texts []string) ([]domain.Vector, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if o.embedding == "" {
		return nil, fmt.Errorf("ollama embed: no embedding model configured")
	}

	var reply ollamaEmbedResponse
	err := o.tr.postJSON(ctx, "/api/embed",
		ollamaEmbedRequest{Model: o.embedding, Input: texts}, &reply)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	if len(reply.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama embed: asked for %d vectors, received %d",
			len(texts), len(reply.Embeddings))
	}

	vectors := make([]domain.Vector, len(texts))
	for i, e := range reply.Embeddings {
		if len(e) == 0 {
			return nil, fmt.Errorf("ollama embed: empty vector for input %d", i)
		}
		vectors[i] = domain.Vector(e)
	}
	return vectors, nil
}

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// Probe answers reachability and model discovery in one call.
//
// Authenticated is reported true whenever the endpoint answers, because there
// is no credential to accept or reject. Reporting false would imply a key is
// missing and send the user looking for one that does not exist.
func (o *Ollama) Probe(ctx context.Context) (ports.ProbeResult, error) {
	var reply ollamaTagsResponse
	if err := o.tr.getJSON(ctx, "/api/tags", &reply); err != nil {
		profile, _ := domain.ProfileByID(domain.ProfileOllama)
		return classifyProbeError(err, profile)
	}

	models := make([]string, 0, len(reply.Models))
	for _, m := range reply.Models {
		models = append(models, m.Name)
	}

	result := ports.ProbeResult{
		Reachable: true, Authenticated: true, Models: models,
		Detail: fmt.Sprintf("Ollama is running, %d model(s) installed", len(models)),
	}
	// Naming the requested model as absent is more useful than a generic
	// success, because the run would otherwise fail later with a 404 that
	// looks like a routing bug.
	if o.model != "" && !containsModel(models, o.model) {
		result.Detail = fmt.Sprintf("Ollama is running but model %q is not installed", o.model)
	}
	return result, nil
}

// containsModel matches Ollama's implicit ":latest" tag, so a user who
// configured "nomic-embed-text" is not told it is missing when
// "nomic-embed-text:latest" is installed.
func containsModel(models []string, want string) bool {
	for _, m := range models {
		if m == want || m == want+":latest" || want == m+":latest" {
			return true
		}
	}
	return false
}
