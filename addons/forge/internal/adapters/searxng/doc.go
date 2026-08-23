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

// Package searxng is the DEVELOPMENT research adapter, not the V1 choice.
//
// FORGE.md 5 mandates exactly one concrete search/fetch adapter in V1 and
// assigns that selection — with the reliability, cost, rate-limit, attribution
// and injection-exposure comparison behind it — to the research-adapter spike,
// #120. That spike has not reported. This package exists so the Research port,
// the evidence shape and the bounded semantics can be exercised end to end
// against a real provider wire format before the choice is made; a self-hosted
// SearXNG instance is convenient for that because it costs nothing per call and
// imposes no rate limit an experiment has to work around. Nothing here should
// be read as a recommendation. When #120 names the V1 adapter, this package is
// either superseded or re-justified on the spike's own terms.
//
// # Scope: search, not fetch
//
// The adapter returns SearXNG's own result snippets. It does not fetch the
// linked pages and does not extract article text, because #120's scope covers
// "bounded query, fetch, content extraction, timeout, size, retry,
// cancellation, and failure semantics" — extraction policy is that spike's
// deliverable, and Go's standard library carries no HTML parser, so building
// one here would mean both inventing the policy and spending the addon's
// dependency budget on it. The consequence is honest and worth stating: snippet
// grounding is thinner than extracted article text, and how much that matters
// is one of the axes #120 has to weigh.
//
// # What the adapter is and is not responsible for
//
// It knows the provider's API, auth and wire format. It does not plan queries,
// does not classify sources, does not decide what is admissible, and does not
// assemble citations. Every returned record leaves domain.SourceQuality at its
// zero value: classification is the source-authority policy's judgement, and an
// adapter that guessed would place candidates beyond the policy's reach — the
// policy would no longer see every candidate it is supposed to rule on.
//
// # Trust boundary
//
// Retrieved titles and snippets are attacker-controlled text. Content is
// carried as domain.UntrustedText, which renders a placeholder under every
// formatting verb and neutralizes fence sentinels when quoted into a prompt.
// The adapter never acts on retrieved text — it does not follow, re-query, or
// interpret it — so a page cannot steer the run that retrieved it.
//
// # Error taxonomy
//
// Three outcomes the pipeline must be able to tell apart:
//
//   - domain.ErrProviderUnreachable — the endpoint never answered (wrong host,
//     service down, DNS failure).
//   - domain.ErrResearchResponse — it answered unusably: a refusing or failing
//     status, an unparseable body, or a body past the size bound.
//   - domain.ErrBudgetExhausted — the attempt ceiling ran out. The exhaustion
//     wraps the last cause as well, so errors.Is answers both questions.
//
// Cancellation and deadlines surface as context.Canceled and
// context.DeadlineExceeded and are never reclassified as any of the three:
// "the caller stopped us" is not a provider fault.
//
// # Operator note
//
// SearXNG serves format=json only when json is listed under search.formats in
// the instance's settings.yml. Until it is, every request answers 403. That is
// the single most common setup mistake with this API, so the adapter names it
// in the error rather than reporting a bare status code.
package searxng
