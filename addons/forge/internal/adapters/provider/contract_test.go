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

package provider_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/provider"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

const testKey = "sk-test-CANARY-not-a-real-key"

type answer struct {
	Answer string `json:"answer"`
}

// capturedRequest records what an adapter actually put on the wire, so a test
// can assert on the request rather than only on the reply.
type capturedRequest struct {
	path    string
	headers http.Header
	body    map[string]any
}

// fakeProvider stands up an httptest server returning a canned reply and
// recording the request. No live calls anywhere in make check.
func fakeProvider(t *testing.T, status int, reply string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path = r.URL.Path
		captured.headers = r.Header.Clone()
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&captured.body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func mustProfile(t *testing.T, id domain.ProfileID) domain.Profile {
	t.Helper()
	p, err := domain.ProfileByID(id)
	if err != nil {
		t.Fatalf("ProfileByID(%q): %v", id, err)
	}
	return p
}

// buildAll returns one client per V1 profile, each pointed at the same fake
// server, so a contract can be asserted across all four in one table.
func buildAll(t *testing.T, endpoint string) map[domain.ProfileID]ports.Provider {
	t.Helper()
	secret := domain.NewSecret(testKey, domain.OriginEnvironment)
	return map[domain.ProfileID]ports.Provider{
		domain.ProfileOpenAI: provider.NewOpenAICompatible(
			mustProfile(t, domain.ProfileOpenAI), "gpt-test", secret, provider.WithEndpoint(endpoint)),
		domain.ProfileOpenRouter: provider.NewOpenAICompatible(
			mustProfile(t, domain.ProfileOpenRouter), "vendor/model", secret, provider.WithEndpoint(endpoint)),
		domain.ProfileAnthropic: provider.NewAnthropic(
			mustProfile(t, domain.ProfileAnthropic), "claude-test", secret, provider.WithEndpoint(endpoint)),
		domain.ProfileOllama: provider.NewOllama(
			mustProfile(t, domain.ProfileOllama), "qwen3:4b", provider.WithEndpoint(endpoint)),
	}
}

// replyFor returns each profile's success shape for the same logical answer,
// so one table can drive all four wire formats.
func replyFor(id domain.ProfileID) string {
	const payload = `{\"answer\":\"42\"}`
	switch id {
	case domain.ProfileAnthropic:
		return `{"content":[{"type":"text","text":"` + payload + `"}],"stop_reason":"end_turn"}`
	case domain.ProfileOllama:
		return `{"message":{"role":"assistant","content":"` + payload + `"},"done":true}`
	default:
		return `{"choices":[{"message":{"role":"assistant","content":"` + payload + `"}}]}`
	}
}

// #123's headline acceptance: all four profiles satisfy ONE port contract.
// Note carefully what this does and does not say — it is the chat contract.
// The embedding contract is a different interface and Anthropic does not
// satisfy it (D-018); the test below asserts that separately and on purpose.
func TestAllFourProfilesSatisfyTheChatContract(t *testing.T) {
	for id := range buildAll(t, "http://unused") {
		t.Run(string(id), func(t *testing.T) {
			srv, captured := fakeProvider(t, http.StatusOK, replyFor(id))
			client := buildAll(t, srv.URL)[id]

			var got answer
			err := client.Generate(context.Background(), ports.Request{
				System: "be terse", User: "what is the answer?",
				Schema: []byte(`{"type":"object"}`), MaxOutputTokens: 100,
			}, &got)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got.Answer != "42" {
				t.Errorf("decoded %+v, want answer 42", got)
			}
			if captured.path == "" {
				t.Error("no request reached the server")
			}
			if identity := client.Identity(); identity.Provider != string(id) {
				t.Errorf("identity provider = %q, want %q", identity.Provider, id)
			}
		})
	}
}

// D-018's compile-time falsifier. Anthropic ships no embeddings API, so its
// adapter must not satisfy Embedder — and if someone later adds a stub that
// pretends otherwise, this fails rather than the pipeline silently believing
// it has vectors.
func TestAnthropicDoesNotSatisfyEmbedder(t *testing.T) {
	clients := buildAll(t, "http://unused")

	if _, ok := provider.EmbedderFor(clients[domain.ProfileAnthropic]); ok {
		t.Error("the Anthropic adapter must NOT implement ports.Embedder: Anthropic ships no embeddings API")
	}
	for _, id := range []domain.ProfileID{domain.ProfileOpenAI, domain.ProfileOpenRouter, domain.ProfileOllama} {
		if _, ok := provider.EmbedderFor(clients[id]); !ok {
			t.Errorf("%s exposes an embeddings API and must implement ports.Embedder", id)
		}
	}
}

// The two representations of the same fact must agree. A profile registry that
// advertised embeddings for an adapter that cannot embed would be a flag
// disagreeing with reality — which is precisely why D-018 made the interface
// the load-bearing form.
func TestProfileEmbeddingFlagAgreesWithTheImplementedInterface(t *testing.T) {
	clients := buildAll(t, "http://unused")
	for _, p := range domain.Profiles() {
		_, implements := provider.EmbedderFor(clients[p.ID])
		if p.Embeds != implements {
			t.Errorf("profile %q advertises Embeds=%v but the adapter implements Embedder=%v",
				p.ID, p.Embeds, implements)
		}
	}
}

// The typed fail-clear path. A caller that needs vectors gets a named
// capability error, not a nil dereference and not a generic failure.
func TestRequiringAnEmbedderFromAnthropicFailsClearly(t *testing.T) {
	clients := buildAll(t, "http://unused")

	_, err := provider.RequireEmbedder(clients[domain.ProfileAnthropic])
	if !errors.Is(err, domain.ErrNoEmbeddingCapability) {
		t.Fatalf("want ErrNoEmbeddingCapability, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "anthropic") {
		t.Errorf("the error should name the provider, got: %v", err)
	}

	if _, err := provider.RequireEmbedder(clients[domain.ProfileOllama]); err != nil {
		t.Errorf("Ollama has embeddings and must not report the capability missing: %v", err)
	}
}
