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

package pack_test

import (
	"testing"

	"github.com/dezeat/golearn/internal/adapters/pack"
	"github.com/dezeat/golearn/internal/domain"
)

const generatedPackYAML = `
pack_version: "0.2.0"
topic:
  slug: go-concurrency
  name: Go Concurrency
generation_spec:
  topic: Go concurrency
  description: goroutines and channels
  count: 1
  difficulty: easy
  style: exam
  language: en
provenance:
  generated_at: 2026-08-23T12:00:00Z
  model:
    provider: ollama
    model: qwen3:8b
  verifier:
    provider: ollama
    model: qwen3:8b
  sources:
    - id: s1
      url: https://example.test/goroutines
      title: Goroutines
  forge_version: 0.3.0
questions:
  - type: single_select
    prompt: Which keyword starts a goroutine?
    choices:
      - {id: a, text: go}
      - {id: b, text: run}
    correct_choice_ids: [a]
    difficulty: easy
    source: "llm:ollama"
    source_ref: s1
    confidence: 0.9
`

// The offline binary must be able to read a pack Forge produced, or generated
// content could not be shared, re-imported, or practiced — which is the whole
// point of a generated pack re-entering the unchanged deterministic pipeline.
func TestGeneratedPackParsesAndValidates(t *testing.T) {
	p, err := pack.NewReader().ReadPackFromBytes([]byte(generatedPackYAML), "generated.yaml")
	if err != nil {
		t.Fatalf("ReadPackFromBytes: %v", err)
	}
	domain.NormalizePack(p)
	if errs := domain.ValidatePack(p, "generated.yaml"); len(errs) != 0 {
		t.Fatalf("a 0.2.0 pack must validate, got %v", errs)
	}

	if p.GenerationSpec == nil {
		t.Fatal("generation_spec was dropped by the parser")
	}
	if p.GenerationSpec.Topic != "Go concurrency" || p.GenerationSpec.Count != 1 {
		t.Errorf("generation_spec parsed wrong: %+v", p.GenerationSpec)
	}
	if p.GenerationSpec.Style != domain.Style("exam") {
		t.Errorf("style = %q, want %q", p.GenerationSpec.Style, "exam")
	}

	if p.Provenance == nil {
		t.Fatal("provenance was dropped by the parser")
	}
	if got := p.Provenance.Model.String(); got != "ollama/qwen3:8b" {
		t.Errorf("model identity = %q", got)
	}
	if len(p.Provenance.Sources) != 1 || p.Provenance.Sources[0].ID != "s1" {
		t.Errorf("source refs parsed wrong: %+v", p.Provenance.Sources)
	}
	if p.Provenance.GeneratedAt.IsZero() {
		t.Error("generated_at was dropped by the parser")
	}
}

// A 0.1.x pack has no pack-level metadata and must keep parsing untouched:
// D-017's whole compatibility promise is that the older schema stays readable.
func TestOriginalSchemaPackStillParsesWithoutMetadata(t *testing.T) {
	const original = `
pack_version: "0.1.0"
topic:
  slug: go
  name: Go
questions:
  - type: single_select
    prompt: Is Go compiled?
    choices:
      - {id: a, text: "yes"}
      - {id: b, text: "no"}
    correct_choice_ids: [a]
`
	p, err := pack.NewReader().ReadPackFromBytes([]byte(original), "original.yaml")
	if err != nil {
		t.Fatalf("ReadPackFromBytes: %v", err)
	}
	domain.NormalizePack(p)
	if errs := domain.ValidatePack(p, "original.yaml"); len(errs) != 0 {
		t.Fatalf("a 0.1.0 pack must still validate, got %v", errs)
	}
	if p.GenerationSpec != nil || p.Provenance != nil {
		t.Error("absent metadata must stay absent, not materialize as empty structs")
	}
}

// The same question in either schema version must produce the same content
// hash, or dedup would treat a 0.2.0 re-import of 0.1.0 content as new.
func TestTheSameQuestionHashesIdenticallyAcrossSchemaVersions(t *testing.T) {
	reader := pack.NewReader()
	generated, err := reader.ReadPackFromBytes([]byte(generatedPackYAML), "generated.yaml")
	if err != nil {
		t.Fatalf("read generated: %v", err)
	}
	const sameQuestionAt010 = `
pack_version: "0.1.0"
topic:
  slug: go-concurrency
  name: Go Concurrency
questions:
  - type: single_select
    prompt: Which keyword starts a goroutine?
    choices:
      - {id: a, text: go}
      - {id: b, text: run}
    correct_choice_ids: [a]
    difficulty: easy
`
	original, err := reader.ReadPackFromBytes([]byte(sameQuestionAt010), "original.yaml")
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	domain.NormalizePack(generated)
	domain.NormalizePack(original)

	generatedHash := domain.ComputeQuestionHash(generated.Topic.Slug, &generated.Questions[0])
	originalHash := domain.ComputeQuestionHash(original.Topic.Slug, &original.Questions[0])
	if generatedHash != originalHash {
		t.Errorf("dedup would break across schema versions:\n 0.1.0: %s\n 0.2.0: %s",
			originalHash, generatedHash)
	}
}
