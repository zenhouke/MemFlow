# Engine Phase-2 Behavior Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add deterministic behavior tests for `core/engine` (Add/Get/Delete/Search/Rebuild) with minimal testability hooks and no business-semantic drift.

**Architecture:** Introduce small internal testability controls (`nowFn`, async compaction control), then build behavior-first tests with lightweight fakes for embedder/estimator/store. Keep assertions centered on public engine behavior; add explicit white-box compatibility tests only for payload recovery helper behavior.

**Tech Stack:** Go (`testing`), existing MemFlow engine/index/retrieval modules, table-driven tests

---

## File Structure Map

- Modify: `core/engine/engine.go` (inject `nowFn` default)
- Modify: `core/engine/add.go` (use `nowFn`, async compaction control)
- Create: `core/engine/test_fakes_test.go` (fake embedder/estimator/store)
- Create: `core/engine/engine_test.go` (constructor/default namespace/rebuild behavior)
- Create: `core/engine/add_get_delete_test.go` (add/get/delete behavior matrix)
- Create: `core/engine/search_test.go` (simple search behavior)

### Task 1: Add minimal testability hooks in engine

**Files:**
- Modify: `core/engine/engine.go`
- Modify: `core/engine/add.go`
- Test: `core/engine/engine_test.go` (later tasks consume hooks)

- [ ] **Step 1: Add `nowFn` field to `MemoryEngine` with production default**

```go
type MemoryEngine struct {
    // ...existing fields...
    nowFn func() time.Time
}

func New(cfg config.Config, embedder embedder.Embedder) *MemoryEngine {
    engine := &MemoryEngine{
        spaces: make(map[string]*MemorySpace),
        embedder: embedder,
        config: cfg,
        nowFn: time.Now,
    }
    // existing init...
}
```

- [ ] **Step 2: Add async compaction control flag**

```go
type MemoryEngine struct {
    // ...
    disableAsyncCompaction bool
}
```

- [ ] **Step 3: Replace time calls in `Add`/`AddDialogues` with `m.nowFn()` where item timestamps are generated**

```go
now := m.nowFn()
```

- [ ] **Step 4: Gate compaction goroutine launch by flag**

```go
if len(space.ShortTerm) >= m.config.CompactionThreshold && !space.IsCompacting {
    if m.disableAsyncCompaction {
        // deterministic test mode: do not spawn goroutine
    } else {
        // existing async path
    }
}
```

- [ ] **Step 5: Run targeted compile/test check**

Run: `go test ./core/engine -run TestMemoryEngine -v`
Expected: compile succeeds, no semantic regressions introduced by hooks.

- [ ] **Step 6: Commit**

```bash
git add core/engine/engine.go core/engine/add.go
git commit -m "test(engine): add deterministic time and compaction hooks"
```

### Task 2: Add reusable deterministic fakes for engine tests

**Files:**
- Create: `core/engine/test_fakes_test.go`

- [ ] **Step 1: Implement fake embedder**

```go
type fakeEmbedder struct {
    vectors map[string][]float64
    fixed   []float64
    err     error
    calls   int
}
```

- [ ] **Step 2: Implement fake estimator**

```go
type fakeEstimator struct {
    value float64
    err   error
    calls int
}
```

- [ ] **Step 3: Implement optional fake vector store**

```go
type fakeVectorStore struct {
    added []vectorstore.VectorRecord
    searchResults []vectorstore.SearchResult
    addErr error
    searchErr error
}
```

- [ ] **Step 4: Add tiny test helper constructors**

```go
func newTestEngine(...) *MemoryEngine { /* deterministic defaults */ }
```

- [ ] **Step 5: Run package tests**

Run: `go test ./core/engine -v`
Expected: PASS for currently implemented tests.

- [ ] **Step 6: Commit**

```bash
git add core/engine/test_fakes_test.go
git commit -m "test(engine): add deterministic fake dependencies"
```

### Task 3: Add constructor/default namespace/rebuild behavior tests

**Files:**
- Create: `core/engine/engine_test.go`

- [ ] **Step 1: Add `New` constructor behavior tests**

```go
func TestNew_SetsDefaults(t *testing.T) {}
```

- [ ] **Step 2: Add default namespace behavior tests using public methods**

```go
func TestDefaultNamespace_Behavior(t *testing.T) {}
```

- [ ] **Step 3: Add `RebuildIndex` behavior tests**

```go
func TestRebuildIndex_DefaultNamespace_NoPanic(t *testing.T) {}
func TestRebuildIndex_MissingNamespace_NoPanic(t *testing.T) {}
func TestRebuildIndex_PreservesCountAndSearchability(t *testing.T) {}
```

- [ ] **Step 4: Run focused tests**

Run: `go test ./core/engine -run 'Test(New|DefaultNamespace|RebuildIndex)' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add core/engine/engine_test.go
git commit -m "test(engine): cover constructor defaults and rebuild behavior"
```

### Task 4: Add Add/Get/Delete behavior tests

**Files:**
- Create: `core/engine/add_get_delete_test.go`

- [ ] **Step 1: Add `Add` routing tests (short vs long term)**

```go
func TestAdd_RoutesByImportanceThreshold(t *testing.T) {}
func TestAdd_EmptyNamespace_UsesDefaultNamespace(t *testing.T) {}
```

- [ ] **Step 2: Add `Add` estimator tests**

```go
func TestAdd_UsesEstimatorWhenImportanceZero(t *testing.T) {}
func TestAdd_EstimatorErrorIsNonFatal(t *testing.T) {}
```

- [ ] **Step 3: Add `Add` embedder error test**

```go
func TestAdd_EmbedderError_NoWrite(t *testing.T) {}
```

- [ ] **Step 4: Add `Get` behavior tests**

```go
func TestGet_MissingNamespace_ReturnsNilNil(t *testing.T) {}
func TestGet_AggregatesAllTiers(t *testing.T) {}
```

- [ ] **Step 5: Add `Delete` behavior tests**

```go
func TestDelete_RemovesFromEachTier(t *testing.T) {}
func TestDelete_MissingNamespace_ReturnsError(t *testing.T) {}
func TestDelete_MissingID_ReturnsError(t *testing.T) {}
```

- [ ] **Step 6: Define and assert delete consistency contract**

```go
// assert deleted ID absent from Get(namespace) and Search(...) in same call path
```

- [ ] **Step 7: Run focused tests**

Run: `go test ./core/engine -run 'Test(Add|Get|Delete)_' -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add core/engine/add_get_delete_test.go
git commit -m "test(engine): add add/get/delete behavior coverage"
```

### Task 5: Add Search simple-path and payload helper compatibility tests

**Files:**
- Create: `core/engine/search_test.go`

- [ ] **Step 1: Add simple-search success tests**

```go
func TestSearch_SimplePath_TopKAndRecencyUpdate(t *testing.T) {}
```

- [ ] **Step 2: Add search error/default-namespace tests**

```go
func TestSearch_EmbedderError(t *testing.T) {}
func TestSearch_DefaultNamespace(t *testing.T) {}
```

- [ ] **Step 3: Add tie-safe ordering assertions**

```go
// assert score-descending only when adjacent scores differ beyond epsilon
```

- [ ] **Step 4: Add white-box compatibility tests for `payloadToMemoryItem`**

```go
func TestPayloadToMemoryItem_FromItemJSON(t *testing.T) {}
func TestPayloadToMemoryItem_FallbackPayload(t *testing.T) {}
func TestPayloadToMemoryItem_InvalidPayload(t *testing.T) {}
```

- [ ] **Step 5: Run focused tests**

Run: `go test ./core/engine -run 'Test(Search|PayloadToMemoryItem)_' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add core/engine/search_test.go
git commit -m "test(engine): add simple search and payload compatibility tests"
```

### Task 6: Phase-2 validation and coverage artifact

**Files:**
- Artifact: `coverage.phase2.out` (generated)

- [ ] **Step 1: Run required Phase-2 command**

Run: `go test ./core/engine -v`
Expected: PASS.

- [ ] **Step 2: Run broader regression check**

Run: `go test ./core/utils ./core/retrieval ./core/index ./core/engine`
Expected: PASS.

- [ ] **Step 3: Run full repository tests (non-blocking signal)**

Run: `go test ./...`
Expected: Prefer PASS; if unrelated failures occur, document them.

- [ ] **Step 4: Generate and verify Phase-2 coverage artifact**

Run: `go test -coverprofile=coverage.phase2.out ./core/engine && test -f coverage.phase2.out`
Expected: coverage file generated and existence check succeeds.

- [ ] **Step 5: Commit policy handling**

```bash
# If user asks to commit artifact:
git add coverage.phase2.out
git commit -m "chore(test): capture phase-2 engine coverage baseline"

# If user says do not commit artifact:
# keep untracked, report generation success only
```

## Acceptance Checklist

- [ ] Minimal hooks added without changing business behavior (`nowFn`, async compaction control).
- [ ] Add/Get/Delete/Search/Rebuild behavior cases implemented per spec.
- [ ] Delete consistency contract validated (same-path Get/Search absence).
- [ ] `payloadToMemoryItem` compatibility tests added as explicit white-box set.
- [ ] Required command passes: `go test ./core/engine -v`.
- [ ] Broader regression command passes: `go test ./core/utils ./core/retrieval ./core/index ./core/engine`.
- [ ] `coverage.phase2.out` generated locally.

## Notes

- Keep assertions behavior-focused; avoid overfitting to private struct internals.
- Avoid strict ordering assertions under tied scores.
- Keep test data small and explicit for readability.
