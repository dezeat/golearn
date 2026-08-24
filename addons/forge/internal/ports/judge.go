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

package ports

import (
	"context"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// DuplicateJudge decides how two questions relate.
//
// It exists as its own port because the decision and the retrieval are
// different capabilities with different costs: [SimilarityIndex] narrows a
// corpus with arithmetic, and this decides a handful of pairs with a provider
// call (D-023). Keeping them apart is what lets the gate bound how many
// judgements it buys without the index knowing that judgements exist.
//
// Implementations must not interpret the verdict — [domain.Relation.IsDuplicate]
// is the project's mapping and is deliberately not exposed to a model.
type DuplicateJudge interface {
	// Judge classifies the relationship between two questions.
	//
	// The order of the arguments must not change the answer. Pairwise LLM
	// judgements are known to move when the order moves, so an implementation
	// that cannot guarantee order-independence is expected to obtain it — for
	// example by confirming a duplicate verdict in the reverse order — rather
	// than to leave the caller to discover it.
	Judge(ctx context.Context, a, b coredomain.PackQuestion) (domain.Relation, error)

	// JudgeIdentity names the model making the decision, so a run record can
	// say what screened a pack. It carries no endpoint.
	JudgeIdentity() coredomain.ModelIdentity
}
