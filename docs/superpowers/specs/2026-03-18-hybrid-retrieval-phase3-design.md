# Hybrid Retrieval Phase-3 Test Design

## Context and Goal

Phase-1 established deterministic unit coverage for `utils/retrieval/index`.
Phase-2 established behavior tests for `engine` core flows (`Add/Get/Delete/Search` simple path).

Phase-3 focuses on `Search` when hybrid retrieval is enabled, validating intent-driven retrieval depth, multi-view fusion (semantic/lexical/symbolic), fallback behavior, and deterministic ranking outcomes.

## Scope

In scope:

- `MemoryEngine.Search` behavior when `EnableHybridSearch = true`
- `HybridRetriever.Search` behavior and retrieval depth (`k`) selection
- Intent analysis interactions relevant to hybrid search path
- Fusion behavior across semantic/lexical/symbolic channels
- Result truncation, ordering expectations, and `LastAccessedAt` updates

Out of scope:

- `Ask` method behavior
- External vector DB integration flows
- Full LLM quality evaluation (content quality of intent reasoning text)

## Approach

Behavior-first tests with deterministic fakes and fixed fixtures.

- Prefer no production changes in Phase-3.
- Reuse existing testability hooks from Phase-2 (`nowFn` etc.).
- Only introduce minimal production changes if deterministic behavior cannot be asserted otherwise.

## Test Architecture

### Files

- Create: `core/engine/hybrid_search_test.go`
- Modify (if needed): `core/engine/test_fakes_test.go` (add fake LLM with call counter)

### Principles

- Table-driven tests with `t.Run`.
- Deterministic embeddings and fixed timestamps.
- Black-box behavior assertions first; avoid over-coupling to internals.
- For ranking, assert strict order only when scores are intentionally non-tied.
- Test naming convention: all Phase-3 tests use `TestHybrid*` prefix.

## Behavior Coverage Matrix

### 1) Hybrid path routing

- With `EnableHybridSearch=true`, search executes hybrid retrieval path.
- Input embedder error still returns early error.

### 2) Intent and retrieval depth (`k`) behavior

- Adaptive mode (`EnableAdaptive=true`):
  - uses `CalculateDynamicK(baseK, complexity, delta)`
  - applies `MinK`/`MaxK` bounds after dynamic-k calculation
- Non-adaptive mode (`EnableAdaptive=false`):
  - uses `intent.RetrievalDepth`
  - does not apply `MinK`/`MaxK` clamp in current implementation
- Intent analysis fallback:
  - LLM failure falls back to rule-based analysis and still returns stable results
- Short query behavior:
  - short query (<=2 words) uses rule-based fast path (LLM call count remains 0)
  - token count rule follows current implementation exactly: `len(strings.Fields(query)) <= 2`
  - boundary fixtures required: 1-token, 2-token, and 3-token queries

#### Normative `k` decision table

`k` must match the following deterministic outcomes:

1. Adaptive lower-bound clamp:
   - config: `baseK=5, delta=2.0, minK=3, maxK=20, adaptive=true`
   - intent: `complexity=0.0`
   - dynamic: `int(5*(1+2*0.0)) = 5`
   - expected final `k=5`

2. Adaptive upper-bound clamp:
   - config: `baseK=5, delta=2.0, minK=3, maxK=8, adaptive=true`
   - intent: `complexity=1.0`
   - dynamic: `int(5*(1+2*1.0)) = 15`
   - expected final `k=8`

3. Adaptive with `delta=0` defaulting behavior:
   - config: `baseK=5, delta=0.0, minK=3, maxK=20, adaptive=true`
   - intent: `complexity=0.5`
   - dynamic uses retrieval default delta `2.0` -> `int(5*(1+2*0.5)) = 10`
   - expected final `k=10`

4. Non-adaptive retrieval depth:
   - config: `adaptive=false`
   - intent: `retrieval_depth=7`
   - expected final `k=7`

### 3) Fusion behavior (semantic/lexical/symbolic)

- Semantic contribution affects ranking when embeddings are distinguishable.
- Lexical contribution affects ranking via BM25 matches.
- Symbolic contribution behavior:
  - with entity/time constraints, matched candidates receive symbolic boost
  - without constraints, baseline symbolic branch applies

Required observable ranking fixtures (black-box):

- Semantic-dominant fixture:
  - weights heavily semantic, query vector near doc-A and far from doc-B
  - expected order: `A` before `B`

- Lexical-dominant fixture:
  - weights heavily lexical, doc-B has stronger BM25 match than doc-A
  - expected order: `B` before `A`

- Symbolic-constraint fixture:
  - query intent includes entity/time constraints matching doc-C but not doc-D
  - expected order: `C` before `D` when symbolic weight is non-trivial

### 4) Output guarantees

- Result count does not exceed selected `k`.
- Ordering is correct for non-tied score fixtures.
- Returned items have `LastAccessedAt` updated to controlled `nowFn` time.
- Persisted side-effect requirement for in-memory candidates:
  - after `Search`, follow-up `Get(namespace)` confirms those returned in-memory items keep updated `LastAccessedAt`

## Fakes and Fixtures

Required test fakes:

- Fake embedder with query/content to vector mapping.
- Fake LLM client:
  - configurable response/error
  - `chatCallCount` for branch verification

Fixture design:

- Use small fixed memory sets with explicit embeddings/text/metadata.
- Avoid accidental score ties unless validating tie-safe assertions.

## Determinism and Stability

- Fixed `nowFn` for all tests.
- Deterministic vectors and document content.
- No network dependencies.
- Avoid asserting map iteration order.

## Validation Commands

Required:

- `go test ./core/engine -run TestHybrid -v`

Recommended:

- `go test ./core/engine -v`
- `go test ./core/utils ./core/retrieval ./core/index ./core/engine`
- `go test ./...`

Coverage artifact:

- `go test -coverprofile=coverage.phase3.out ./core/engine`

## Success Criteria

- Hybrid search behavior tests added and deterministic.
- `k` selection behavior verified for adaptive and non-adaptive modes.
- Intent fallback and short-query fast path behavior validated.
- Fusion channel effects validated with deterministic fixtures.
- Required command passes: `go test ./core/engine -run TestHybrid -v`.
- Phase-3 coverage artifact can be generated locally.

## Risks and Mitigations

- Risk: brittle ranking assertions under close scores.
  - Mitigation: craft fixtures with clear score separation; tie-safe assertions otherwise.

- Risk: over-testing internals rather than behavior.
  - Mitigation: assert returned items/order/count/timestamps and call counts only.

- Risk: hidden nondeterminism from evolving defaults.
  - Mitigation: explicitly set hybrid config values in each test.

## Next Phase Preview

- Extend to `Ask` behavior and context assembly.
- Add targeted integration tests around external store + hybrid interaction.
