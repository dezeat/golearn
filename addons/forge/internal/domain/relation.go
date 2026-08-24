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

import "fmt"

// Relation is how two questions relate to one another.
//
// The taxonomy is the labeled fixture set's own
// (addons/forge/internal/app/testdata/similarity_pairs.json), assigned to
// questions before any model scored them. It is reused here rather than
// invented so the judge is measured against the same vocabulary it produces
// (D-023, FORGE-EXPERIMENTS A-25).
type Relation string

const (
	// RelationIdentical is the same assessment in the same wording. Choice
	// ids, choice order and formatting do not matter.
	RelationIdentical Relation = "identical"
	// RelationParaphrase is the same assessment reworded, with overlapping
	// vocabulary.
	RelationParaphrase Relation = "paraphrase"
	// RelationSemantic is the same assessment reworded with almost no shared
	// vocabulary. These are the pairs no embedding model separated (A-22,
	// A-24) and the reason this taxonomy is decided by a judge.
	RelationSemantic Relation = "semantic"
	// RelationCompetency is the same concept with a different competency
	// tested. Permitted.
	RelationCompetency Relation = "competency"
	// RelationConcept is the same topic with a different concept asked about.
	// Permitted, and the class the hardest fixture negative belongs to.
	RelationConcept Relation = "concept"
	// RelationUnrelated is a different topic. Permitted.
	RelationUnrelated Relation = "unrelated"
)

// duplicateRelations maps the taxonomy onto the gate's verdict.
//
// The mapping belongs to the project and predates any measurement; it is never
// shown to a judge, which is asked only to classify. A model that has learned
// to answer "unrelated" when unsure must not be able to score well by guessing
// the verdict rather than the relation.
var duplicateRelations = map[Relation]bool{
	RelationIdentical:  true,
	RelationParaphrase: true,
	RelationSemantic:   true,
	RelationCompetency: false,
	RelationConcept:    false,
	RelationUnrelated:  false,
}

// IsDuplicate reports whether the gate must act on this relation.
//
// An unknown relation is not a duplicate, but callers are expected to reject
// it via [ParseRelation] rather than rely on that: silently treating an
// unrecognised label as "fine" is how a changed taxonomy would disable the
// gate without failing anything.
func (r Relation) IsDuplicate() bool { return duplicateRelations[r] }

// Valid reports whether r is part of the taxonomy.
func (r Relation) Valid() bool {
	_, ok := duplicateRelations[r]
	return ok
}

// Relations returns the taxonomy in a fixed order, for prompts and schemas
// that must enumerate it.
//
// Fixed rather than map order, because a prompt that reorders between runs is
// a prompt that cannot be reproduced.
func Relations() []Relation {
	return []Relation{
		RelationIdentical, RelationParaphrase, RelationSemantic,
		RelationCompetency, RelationConcept, RelationUnrelated,
	}
}

// ParseRelation converts a model's answer into a Relation, refusing anything
// outside the taxonomy.
//
// Refusing rather than defaulting follows the same rule as the rest of this
// package: Cosine refuses a dimension mismatch, the index refuses a mixed-model
// search, and D-014 refuses an unrecognised schema. A label nobody defined is
// not evidence about two questions.
func ParseRelation(s string) (Relation, error) {
	r := Relation(s)
	if !r.Valid() {
		return "", fmt.Errorf("%w: %q is not one of the six relations", ErrUnknownRelation, s)
	}
	return r, nil
}
