# Engine Phase-2 Behavior Test Design

## Context and Goal

Phase-1 established deterministic unit tests for `core/utils`, `core/retrieval`, and `core/index`.

Phase-2 extends test coverage to `core/engine` with a behavior-first strategy focused on `Add`, `Get`, `Delete`, and `Search` while allowing minimal production changes only for testability controls. A small internal helper compatibility set is included explicitly and labeled as white-box.

## Scope

In scope:

- Engine behavior tests for:
  - `MemoryEngine.Add`
  - `MemoryEngine.Get`
  - `MemoryEngine.Delete`
  - `MemoryEngine.Search` (simple path first)
  - `MemoryEngine.RebuildIndex`
- Internal compatibility tests for `payloadToMemoryItem` recovery paths (white-box)
- Minimal production testability hooks that do not alter business semantics.

Out of scope:

- Deep hybrid retrieval ranking verification (covered in a later phase)
- End-to-end external vector DB integration
- Performance/benchmark testing

## Approach and Rationale

Chosen approach: behavior-first tests with minimal, explicit testability hooks.

Why:

- Keeps tests robust and deterministic (no flaky async/time dependencies).
- Prioritizes public observable behavior over internal implementation details.
- Preserves production semantics while reducing test friction.

## Minimal Production Changes (Testability Only)

Allowed changes:

1. Inject time source into `MemoryEngine`:
   - Add `nowFn func() time.Time` field with default `time.Now`.
   - Use `m.nowFn()` in places currently calling `time.Now()` where behavior assertions depend on time.

2. Control asynchronous compaction in tests:
   - Add a minimal internal flag (e.g., `disableAsyncCompaction bool`).
   - When enabled in tests, skip goroutine launch and keep deterministic behavior.

Non-goals for these hooks:

- No production API behavior changes.
- No refactor of business rules.
- No new feature logic.

## Test Architecture

### File layout

- `core/engine/engine_test.go`
  - constructor defaults, namespace defaults, index rebuild safety
- `core/engine/add_get_delete_test.go`
  - add/get/delete behavior matrix
- `core/engine/search_test.go`
  - simple search behavior and failure paths
- `core/engine/test_fakes_test.go`
  - fake embedder/estimator/store helpers used across tests

### Design principles

- Table-driven tests + `t.Run` subtests.
- Deterministic fixtures (fixed embeddings, fixed timestamps).
- Public behavior assertions first; minimal white-box checks only when necessary.
- No network or real external service dependencies.

## Detailed Behavior Coverage Matrix

### 1) Add

- Empty namespace routes to `default`.
- Threshold routing:
  - `importance >= LongTermImportanceThreshold` -> long-term tier
  - otherwise -> short-term tier
- Estimator path:
  - when input importance is `0` and estimator exists, estimator result drives tier routing
  - when estimator returns error, add flow is non-fatal and continues with existing importance value
- Embed failure path:
  - embedder error returns error and does not write memory item

### 2) Get

- Missing namespace returns `nil, nil`.
- Existing namespace returns aggregated items from short/long/archived.
- Aggregation includes all existing items with expected count and identity.

### 3) Delete

- Deletes item from:
  - short-term
  - long-term
  - archived
- Namespace missing -> expected error.
- ID missing in existing namespace -> expected error.
- Post-delete behavior confirms item is no longer retrievable via engine behavior:
  - deleted ID absent from `Get(namespace)`
  - deleted ID absent from `Search(...)` results for deterministic fixture queries
  - consistency contract: deletion is immediately search-consistent in the same call path (no extra sync step)

### 4) Search (simple path)

- Embedder error -> returns error.
- Returns no more than `TopK` items.
- Result order is score-descending for differing scores.
- Tie handling in tests: do not assert strict relative order when scores are equal.
- Returned items receive updated `LastAccessedAt` using controlled `nowFn`.
- Empty namespace maps to `default` behavior.

### 5) Rebuild behavior

- `RebuildIndex` on existing/default namespace is safe and does not panic.
- `RebuildIndex` on missing namespace is a no-op and does not panic.
- Observable post-conditions after rebuild:
  - item count from `Get(namespace)` remains unchanged
  - deterministic fixture IDs remain searchable via `Search(...)`

### 6) Internal payload recovery helper compatibility (white-box)

These tests are explicitly white-box compatibility checks, not primary behavior tests.

- `payloadToMemoryItem`:
  - full JSON payload path restores item
  - fallback payload path restores minimal item
  - invalid payload returns `nil` as expected

## Fakes and Dependency Strategy

Provide deterministic fakes:

- Fake embedder:
  - configurable embedding result or error
  - optional call count for assertion
- Fake estimator:
  - configurable importance result or error
- Fake vector store (only if needed for behavior path):
  - records add/search calls
  - returns fixed search results

No real LLM or vector DB dependency in Phase-2 tests.

## Determinism and Flake Prevention

- Fixed `nowFn` in tests.
- Disable async compaction in tests.
- Avoid map-order assertions; normalize unordered outputs where needed.
- Keep floating-point comparisons tolerant where score values are asserted.

## Execution Plan (Validation Commands)

Required for Phase-2 completion:

- `go test ./core/engine -v`

Recommended safety checks:

- `go test ./core/utils ./core/retrieval ./core/index ./core/engine`
- `go test ./...`

Coverage artifact for Phase-2 baseline:

- `go test -coverprofile=coverage.phase2.out ./core/engine`

## Success Criteria

- Behavior tests exist for all in-scope engine methods.
- Deterministic testability hooks are implemented with no semantic behavior drift.
- `go test ./core/engine -v` passes.
- No regressions in previously covered modules when running broader test commands.
- `coverage.phase2.out` can be generated locally (artifact commit policy decided separately).

## Risks and Mitigations

- Risk: tests become coupled to internals.
  - Mitigation: assert via public API behavior and observable outputs.

- Risk: async compaction introduces nondeterminism.
  - Mitigation: test-only async control hook.

- Risk: time-dependent assertions are flaky.
  - Mitigation: injectable `nowFn` and fixed timestamps in tests.

## Next Phase Preview

After Phase-2 stabilization:

- Extend hybrid retrieval behavior tests with intent-driven path assertions.
- Add targeted integration tests for vector store interaction boundaries.
