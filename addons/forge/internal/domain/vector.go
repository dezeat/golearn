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

package domain

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"

	coredomain "github.com/dezeat/golearn/internal/domain"
)

// Vector is an embedding.
//
// float32 rather than float64 is deliberate. Embedding models emit float32,
// so float64 would store converted precision that never existed, at twice the
// bytes — and D-017's storage plan puts these in a SQLite BLOB alongside a
// user's questions. See docs/design/FORGE-EXPERIMENTS.md Part C for the
// measured bytes-per-1000-questions this buys.
type Vector []float32

// Dim reports the dimensionality.
func (v Vector) Dim() int { return len(v) }

// Cosine returns the cosine similarity of two vectors, in [-1, 1].
//
// It reports an error rather than a sentinel score on mismatched or empty
// input: a silent 0 would read as "unrelated" and let a dimension mismatch —
// which means the two vectors came from different embedding models and are not
// comparable at all — pass as a legitimate similarity verdict.
//
// The accumulation is float64 even though the inputs are float32, because
// summing several hundred float32 products in float32 loses enough precision
// to move a score across a threshold.
func Cosine(a, b Vector) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("cosine: dimension mismatch (%d vs %d): vectors from different embedding models are not comparable", len(a), len(b))
	}
	if len(a) == 0 {
		return 0, fmt.Errorf("cosine: empty vectors")
	}

	var dot, normA, normB float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		normA += x * x
		normB += y * y
	}
	if normA == 0 || normB == 0 {
		return 0, fmt.Errorf("cosine: zero-magnitude vector")
	}

	score := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	// NaN must be an error, not a score. Every comparison against NaN is
	// false, so a NaN-scoring neighbor reads as *below* any threshold — the
	// similarity gate would silently wave a duplicate through, and a duplicate
	// missing from the results is indistinguishable from no duplicate at all.
	// It also makes a score comparator inconsistent (NaN != NaN), which turns
	// sorting into implementation-defined order and breaks the determinism law.
	//
	// A NaN reaches here from a corrupted or truncated BLOB: the decoder
	// accepts any four-byte group, and several of them are NaN bit patterns.
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return 0, fmt.Errorf("cosine: vectors contain a non-finite component; the stored embedding is corrupt")
	}
	return score, nil
}

// MarshalVector encodes a vector for BLOB storage as little-endian float32.
//
// Explicit little-endian rather than the host's byte order: a database file is
// a portable artifact, and a user moving one between architectures must not
// silently read their embeddings back as noise.
func MarshalVector(v Vector) []byte {
	out := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(f))
	}
	return out
}

// UnmarshalVector decodes a BLOB written by [MarshalVector].
func UnmarshalVector(b []byte) (Vector, error) {
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("unmarshal vector: %d bytes is not a whole number of float32", len(b))
	}
	v := make(Vector, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v, nil
}

// canonicalSeparator keeps distinct fields from colliding when concatenated:
// without it, a prompt ending in a word and an option starting with one would
// join into a token that appears in neither.
const canonicalSeparator = "\n"

// CanonicalText renders the representation the similarity gate compares.
//
// FORGE.md 7 fixes the inputs: normalized question prompt, answer options,
// correct answer(s), and tags. Introductions and explanations are excluded
// because they share source language across questions drawn from the same
// evidence and drive false positives — and the gate's quality target favors
// few false positives.
//
// Options and tags are sorted, and correct ids sorted, so that two candidates
// differing only in presentation order produce identical text. That mirrors
// D-007's treatment of correct ids and keeps the representation stable for the
// same reason: order here is presentation, not content.
func CanonicalText(q *coredomain.PackQuestion) string {
	var b strings.Builder
	b.WriteString(coredomain.NormalizeText(q.Prompt))

	options := make([]string, 0, len(q.Choices))
	correctByID := make(map[string]bool, len(q.CorrectChoiceIDs))
	for _, id := range q.CorrectChoiceIDs {
		correctByID[strings.TrimSpace(id)] = true
	}
	for _, c := range q.Choices {
		text := coredomain.NormalizeText(c.Text)
		// Mark correctness on the option itself rather than listing correct
		// ids separately: ids are arbitrary labels, so two packs asking the
		// same question with different id schemes would otherwise look
		// different to the gate.
		if correctByID[strings.TrimSpace(c.ID)] {
			text = "+ " + text
		} else {
			text = "- " + text
		}
		options = append(options, text)
	}
	sort.Strings(options)

	tags := make([]string, 0, len(q.Tags))
	for _, t := range q.Tags {
		tags = append(tags, strings.ToLower(strings.TrimSpace(t)))
	}
	sort.Strings(tags)

	for _, o := range options {
		b.WriteString(canonicalSeparator)
		b.WriteString(o)
	}
	for _, t := range tags {
		b.WriteString(canonicalSeparator)
		b.WriteString(t)
	}
	return b.String()
}
