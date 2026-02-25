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
	"os"
	"path/filepath"
	"testing"

	"github.com/dezeat/golearn/internal/adapters/pack"
)

func TestReader_ReadPackYAML(t *testing.T) {
	content := `
pack_version: "0.1.0"
topic:
  slug: "test-topic"
  name: "Test Topic"
questions:
  - type: "single_select"
    prompt: "What is 1+1?"
    choices:
      - { id: "A", text: "1" }
      - { id: "B", text: "2" }
    correct_choice_ids: ["B"]
    tags: ["math"]
    difficulty: easy
`
	p := filepath.Join(t.TempDir(), "test.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	r := pack.NewReader()
	pk, err := r.ReadPack(p)
	if err != nil {
		t.Fatalf("ReadPack() error: %v", err)
	}
	if pk.Topic.Slug != "test-topic" {
		t.Errorf("topic slug = %q, want %q", pk.Topic.Slug, "test-topic")
	}
	if len(pk.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(pk.Questions))
	}
	if pk.Questions[0].Prompt != "What is 1+1?" {
		t.Errorf("prompt = %q", pk.Questions[0].Prompt)
	}
}

func TestReader_ReadPackJSON(t *testing.T) {
	content := `{
  "pack_version": "0.1.0",
  "topic": {"slug": "json-topic", "name": "JSON Topic"},
  "questions": [
    {
      "type": "single_select",
      "prompt": "Is JSON valid?",
      "choices": [{"id": "A", "text": "Yes"}, {"id": "B", "text": "No"}],
      "correct_choice_ids": ["A"],
      "tags": ["json"],
      "difficulty": "easy"
    }
  ]
}`
	p := filepath.Join(t.TempDir(), "test.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	r := pack.NewReader()
	pk, err := r.ReadPack(p)
	if err != nil {
		t.Fatalf("ReadPack() error: %v", err)
	}
	if pk.Topic.Slug != "json-topic" {
		t.Errorf("topic slug = %q", pk.Topic.Slug)
	}
	if len(pk.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(pk.Questions))
	}
}

func TestReader_ReadPackYML(t *testing.T) {
	content := `pack_version: "0.1.0"
topic:
  slug: "yml"
  name: "YML"
questions:
  - type: "single_select"
    prompt: "YML extension?"
    choices:
      - { id: "A", text: "Yes" }
      - { id: "B", text: "No" }
    correct_choice_ids: ["A"]
`
	p := filepath.Join(t.TempDir(), "test.yml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r := pack.NewReader()
	pk, err := r.ReadPack(p)
	if err != nil {
		t.Fatalf("ReadPack(.yml) error: %v", err)
	}
	if pk.Topic.Slug != "yml" {
		t.Errorf("slug = %q", pk.Topic.Slug)
	}
}

func TestReader_ReadPackUnsupportedExtension(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := pack.NewReader()
	_, err := r.ReadPack(p)
	if err == nil {
		t.Fatal("expected error for .txt extension")
	}
}

func TestReader_ReadPackMissingFile(t *testing.T) {
	r := pack.NewReader()
	_, err := r.ReadPack("/tmp/nonexistent-golearn-test.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReader_ReadPackInvalidYAML(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(p, []byte(":\n  :\n    - [invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := pack.NewReader()
	_, err := r.ReadPack(p)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestReader_ReadDirectory(t *testing.T) {
	dir := t.TempDir()
	yaml1 := `pack_version: "0.1.0"
topic: {slug: "a", name: "A"}
questions:
  - type: "single_select"
    prompt: "Q1"
    choices: [{id: "A", text: "X"}, {id: "B", text: "Y"}]
    correct_choice_ids: ["A"]
`
	yaml2 := `pack_version: "0.1.0"
topic: {slug: "b", name: "B"}
questions:
  - type: "single_select"
    prompt: "Q2"
    choices: [{id: "A", text: "X"}, {id: "B", text: "Y"}]
    correct_choice_ids: ["B"]
`
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(yaml1), 0o644); err != nil {
		t.Fatalf("write a.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.yaml"), []byte(yaml2), 0o644); err != nil {
		t.Fatalf("write b.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatalf("write readme.txt: %v", err)
	}

	r := pack.NewReader()
	packs, errs := r.ReadDirectory(dir)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(packs) != 2 {
		t.Fatalf("expected 2 packs, got %d", len(packs))
	}
}

func TestReader_ReadDirectoryMissing(t *testing.T) {
	r := pack.NewReader()
	_, errs := r.ReadDirectory("/tmp/nonexistent-golearn-dir-test")
	if len(errs) == 0 {
		t.Fatal("expected error for missing directory")
	}
}
