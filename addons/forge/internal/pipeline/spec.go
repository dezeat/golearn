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

package pipeline

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// Request bounds. A spec outside them cannot succeed, so it is refused before
// a single provider call is spent.
const (
	// MinCount is one: a pack of zero questions is not a pack.
	MinCount = 1
	// MaxCount is a deliberate product bound rather than a technical one. The
	// chain makes several model calls per question, and on modest hardware a
	// large pack is an unattended overnight job presented as an interactive
	// feature (FORGE-EXPERIMENTS B-2.1).
	MaxCount = 20
	// MaxTopicLength keeps a topic to something a prompt and a pack header can
	// carry.
	MaxTopicLength = 200
	// MaxDescriptionLength bounds the optional free-text steer.
	MaxDescriptionLength = 1000
)

// ValidateSpec checks a generation request before any work begins.
func ValidateSpec(spec domain.GenerationSpec) error {
	topic := strings.TrimSpace(spec.Topic)
	switch {
	case topic == "":
		return fmt.Errorf("generation spec: a topic is required")
	case len(topic) > MaxTopicLength:
		return fmt.Errorf("generation spec: topic is %d characters, the limit is %d",
			len(topic), MaxTopicLength)
	case len(spec.Description) > MaxDescriptionLength:
		return fmt.Errorf("generation spec: description is %d characters, the limit is %d",
			len(spec.Description), MaxDescriptionLength)
	case spec.Count < MinCount:
		return fmt.Errorf("generation spec: count must be at least %d, got %d", MinCount, spec.Count)
	case spec.Count > MaxCount:
		return fmt.Errorf("generation spec: count is %d, the limit is %d", spec.Count, MaxCount)
	}

	if spec.Difficulty != coredomain.DifficultyUnset && !coredomain.ValidDifficulties[spec.Difficulty] {
		return fmt.Errorf("generation spec: difficulty %q must be easy, medium or hard", spec.Difficulty)
	}
	// Style is deliberately unvalidated: the intent vocabulary is spike-gated
	// (#105) and validating it here would create the circular dependency #121
	// forbids. An unknown style is content-neutral, not an error.
	if TopicSlug(topic) == "" {
		return fmt.Errorf("generation spec: topic %q contains no characters usable in a topic identifier", spec.Topic)
	}
	return nil
}

var nonSlugRunes = regexp.MustCompile(`[^a-z0-9]+`)

// TopicSlug derives a stable kebab-case topic identifier from a topic name.
//
// The slug is a data contract, not presentation: it feeds the topic upsert on
// import and the D-007 content hash, so two runs on the same topic must
// produce the same slug or the library would grow a duplicate topic per run
// and dedup would stop working across them.
func TopicSlug(topic string) string {
	slug := nonSlugRunes.ReplaceAllString(strings.ToLower(strings.TrimSpace(topic)), "-")
	return strings.Trim(slug, "-")
}
