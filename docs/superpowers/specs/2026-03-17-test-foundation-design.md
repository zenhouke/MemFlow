# Test Foundation Design for MemFlow (Phase 1)

## Context and Goal

MemFlow currently has no automated tests. The immediate goal is to establish a stable, low-friction unit test foundation that protects core deterministic logic and enables safe refactoring.

This phase intentionally avoids external systems (LLM providers, vector DB services, network I/O) and focuses on pure or near-pure logic that can be validated quickly and reliably.

## Scope

In scope (Phase 1):

- `core/utils/common.go`
- `core/utils/string.go`
- `core/utils/vector.go`
- `core/retrieval/intent.go`
- `core/index/metadata.go`
- `core/index/bm25.go`

Out of scope (Phase 1):

- End-to-end examples
- External LLM integration behavior
- Engine-level cross-component integration (`core/engine/*`)

## Approach Options Considered

1. Progressive unit-first strategy (recommended)
   - Start with deterministic utility and index/query logic.
   - Pros: fast adoption, stable signals, low mocking cost.
   - Cons: full business-flow confidence comes in later phases.

2. Engine-first behavior testing
   - Start directly from `core/engine` add/search/get/delete flows.
   - Pros: closer to user-visible behavior.
   - Cons: heavier setup and mock complexity on day one.

3. Example-based smoke testing first
   - Validate examples as integration probes.
   - Pros: quick runnable confidence.
   - Cons: coarse failure localization and weaker regression precision.

Chosen approach: Option 1.

## Test Architecture

### File placement

- Keep `_test.go` files colocated with source files.
- Use package-local tests (same package) for direct function coverage.

### Test style

- Prefer table-driven tests with `t.Run` subtests.
- Use explicit scenario names for readability.
- Keep each case focused on one behavior.

### Assertion strategy

- Use standard library assertions (`if got != want`, `t.Fatalf`).
- For float comparisons, use epsilon-based checks.
- Avoid assertions that depend on map iteration order.
- For result ordering, only assert strict order when the production code guarantees it.
- When score/order ties are possible, compare as sets or sort by deterministic secondary keys in test helpers.

### Determinism rules

- No network calls.
- No time-sensitive flakiness; use fixed timestamps.
- Compare unordered outputs via set-like comparison or sort first.

## Component Design

### 1) `core/utils`

#### `common.go`

- `ToFloat32`: empty input, normal input, mixed sign values.
- `Max` and `Min`: branch coverage for greater/less/equal.

#### `string.go`

- `UniqueStrings`: deduplication, empty-string filtering, order preservation.

#### `vector.go`

- `Cosine`: mismatch length, empty vectors, zero norm vectors, identical vectors, orthogonal vectors.
- `ExpDecay`: lambda zero, t zero, normal decay behavior.

### 2) `core/retrieval/intent.go`

#### `NewQueryAnalyzer`

- Ensure non-positive `baseK` falls back to default `5`.

#### `Analyze`

- Without `llmClient`: rule-based path.
- With `llmClient` and short query: verify fast path avoids LLM call.

#### `parseIntentResponse`

- Valid raw JSON parsing.
- JSON embedded in surrounding text.
- Invalid JSON fallback to rule-based result.
- `retrieval_depth == 0` fallback to `baseK`.

#### Rule helpers

- `ruleBasedAnalyze` branch coverage for factual/temporal/reasoning/aggregation/default.
- `extractKeywords`, `extractEntities`, keyword detectors, dynamic K function.

### 3) `core/index/metadata.go`

- Add and direct index search (`SearchByEntity`, `SearchByTopic`) with case normalization.
- Composite metadata `Search` with entity/topic/tag filters.
- Time filtering via `TimeStart/TimeEnd` and inclusive boundary checks.
- Delete behavior: remove from metadata maps and inverted indexes.
- `GetMetadata` nil/non-nil behavior.

### 4) `core/index/bm25.go`

- Tokenization normalization (case, punctuation, stop-words).
- Add/Delete internal state transitions (`docCount`, `avgDocLen`, `docFreq`).
- Search behavior: empty index, score ordering, topK truncation.
- Validate deleted docs no longer influence search output.

## Data Flow and Dependency Strategy

- Unit tests execute entirely in-process.
- `intent` tests use a minimal fake `llm.LLMClient` to control response and call count.
- No persistent storage or external service dependencies.

### Fake LLM test contract (`core/retrieval/intent.go`)

- Implement a test-only fake with:
  - a configurable `response string`
  - a configurable `err error`
  - a `chatCallCount int` counter
- `Chat(ctx, messages)` returns configured `(response, err)` and increments `chatCallCount`.
- Required assertions:
  - short-query fast path (`<= 2` words) keeps `chatCallCount == 0`
  - non-short query with configured success sets `chatCallCount == 1`
  - non-short query with configured error falls back to rule-based intent

## Error Handling and Edge Cases

- Empty and malformed inputs are explicit first-class cases.
- Fallback logic in query-intent parsing is validated as expected behavior.
- Boundary conditions for time and float logic are covered.

## Test Execution Plan

1. Add test files in target directories.
2. Run scoped required checks for Phase 1:
   - `go test ./core/utils ./core/retrieval ./core/index`
3. Run full-repo check as non-blocking signal:
   - `go test ./...`
4. Ensure all new tests pass and remain deterministic.
5. Capture baseline coverage report artifact:
   - `go test -coverprofile=coverage.phase1.out ./core/utils ./core/retrieval ./core/index`

## Success Criteria

- Deterministic tests added for all in-scope files.
- Required scoped test command passes:
  - `go test ./core/utils ./core/retrieval ./core/index`
- Critical branch checklist is fully covered:
  - `Cosine` early-return branches (len mismatch, empty, zero norm)
  - `QueryAnalyzer.Analyze` paths (nil LLM fallback, short-query fast path, LLM error fallback)
  - `parseIntentResponse` paths (raw JSON, embedded JSON, invalid JSON fallback, zero retrieval depth fallback)
  - `MetadataIndex.Search` paths (filter-based candidates, no-filter all-docs path, time filtering)
  - `BM25Index.Search` paths (empty index, positive-scored docs only, topK truncation)
- Baseline coverage artifact exists at project root as `coverage.phase1.out`.
- A repeatable foundation exists for Phase 2 (`core/engine` behavior tests).

## Risks and Mitigations

- Risk: overfitting tests to implementation details.
  - Mitigation: prefer behavior assertions over private-state assumptions.

- Risk: flaky assertions from unordered results.
  - Mitigation: compare sorted or set-normalized outputs.

- Risk: float precision instability.
  - Mitigation: epsilon comparisons.

## Next Phase Preview

After Phase 1 is complete and stable:

- Add engine-level behavior tests for add/search/get/delete.
- Introduce controlled mocks around embedder/llm/vectorstore boundaries.
- Optionally add CI coverage threshold and race-test stage.
