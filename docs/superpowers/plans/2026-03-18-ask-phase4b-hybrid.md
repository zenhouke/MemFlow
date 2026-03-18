# Ask Phase-4b (Hybrid Mode) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add deterministic `Ask` tests in hybrid mode, covering success/prompt contract/source inclusion/error propagation/empty-result fallback without re-testing full hybrid internals.

**Architecture:** Extend `core/engine/ask_test.go` with `TestAskHybrid_*` cases using existing fakes. Keep assertions behavior-level (returned value/error, call counts, message contract, prompt fragments), and use deterministic fixtures with explicit hybrid-mode setup.

**Tech Stack:** Go (`testing`), MemFlow `core/engine`

---

## File Structure Map

- Modify: `core/engine/ask_test.go`
- Reuse: `core/engine/test_fakes_test.go`

### Task 1: Add hybrid Ask test setup helpers

**Files:**
- Modify: `core/engine/ask_test.go`

- [ ] **Step 1: Add helper to enforce hybrid mode in Ask tests**

```go
func newAskTestEngineHybrid(embed *fakeEmbedder) *MemoryEngine {
    cfg := newTestConfig()
    cfg.EnableHybridSearch = true
    // explicit hybrid config defaults for deterministic behavior
    return newEngineWithConfigAndNow(cfg, embed, fixedNow)
}
```

- [ ] **Step 2: Add helper assertions for captured Ask message contract**

```go
// assert exactly 2 messages, role order system->user,
// non-empty system content, required user prompt sections
```

- [ ] **Step 3: Run sanity command**

Run: `go test ./core/engine -run TestAskHybrid -v`
Expected: compile pass (may have no tests yet).

- [ ] **Step 4: Commit**

```bash
git add core/engine/ask_test.go
git commit -m "test(engine): add ask hybrid test setup helpers"
```

### Task 2: Add hybrid success and prompt-contract tests

**Files:**
- Modify: `core/engine/ask_test.go`

- [ ] **Step 1: Add hybrid success passthrough test**

```go
func TestAskHybrid_Success_ReturnsLLMAnswer(t *testing.T) {}
```

- [ ] **Step 2: Add hybrid prompt-contract test**

```go
func TestAskHybrid_PromptContainsRequiredFragments(t *testing.T) {
    // Memory Context, [1], Question, Answer fragments
    // message contract assertions
    // explicit assertion: at least one known hybrid-retrieved memory content substring exists in user prompt
}
```

- [ ] **Step 3: Run focused tests**

Run: `go test ./core/engine -run 'TestAskHybrid_(Success_ReturnsLLMAnswer|PromptContainsRequiredFragments)$' -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add core/engine/ask_test.go
git commit -m "test(engine): add ask hybrid success and prompt contract tests"
```

### Task 3: Add hybrid source-line and empty-result tests

**Files:**
- Modify: `core/engine/ask_test.go`

- [ ] **Step 1: Add source-line inclusion test in hybrid flow**

```go
func TestAskHybrid_PromptIncludesSourceWhenOriginalContentPresent(t *testing.T) {}
```

- [ ] **Step 2: Add empty-result fallback test in hybrid flow**

```go
func TestAskHybrid_NoRelevantMemories_ReturnsFallback(t *testing.T) {
    // exact fallback text
    // llm chat not called
}
```

- [ ] **Step 3: Run focused tests**

Run: `go test ./core/engine -run 'TestAskHybrid_(PromptIncludesSourceWhenOriginalContentPresent|NoRelevantMemories_ReturnsFallback)$' -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add core/engine/ask_test.go
git commit -m "test(engine): add ask hybrid source and empty-result tests"
```

### Task 4: Add hybrid error-path tests

**Files:**
- Modify: `core/engine/ask_test.go`
- Modify (minimal, if required): `core/engine/engine.go`, `core/engine/search.go`

- [ ] **Step 1: Add embedder error propagation test**

```go
func TestAskHybrid_SearchEmbedderError_Propagates(t *testing.T) {
    // Ask returns embedder/search error unchanged
    // llm chat not called
}
```

- [ ] **Step 2: Add hybrid retrieval failure propagation test**

```go
func TestAskHybrid_HybridRetrievalError_Propagates(t *testing.T) {
    // use test-only hybrid-search override hook to return a sentinel error
    // assert unchanged error
    // assert llm chat count == 0
}
```

- [ ] **Step 2.1: Add minimal test-only hybrid override hook if needed**

```go
// in MemoryEngine:
// hybridSearchOverride func(ctx context.Context, query string, queryEmbedding []float64, space *MemorySpace, now time.Time) ([]*MemoryItem, error)

// in Search():
// if m.config.EnableHybridSearch {
//   if m.hybridSearchOverride != nil { return m.hybridSearchOverride(...) }
//   return m.hybridSearch(...)
// }
```

- [ ] **Step 2.2: Validate override default-path safety**

Run: `go test ./core/engine -run 'Test(Hybrid|AskHybrid)' -v`
Expected: existing hybrid tests still pass when override is nil.

- [ ] **Step 3: Add LLM failure after successful hybrid search test**

```go
func TestAskHybrid_LLMError_Propagates(t *testing.T) {
    // search succeeds
    // llm chat returns error
    // Ask returns same error
}
```

- [ ] **Step 5: Run focused tests**

Run: `go test ./core/engine -run 'TestAskHybrid_(SearchEmbedderError_Propagates|HybridRetrievalError_Propagates|LLMError_Propagates)$' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add core/engine/ask_test.go core/engine/engine.go core/engine/search.go
git commit -m "test(engine): add ask hybrid error propagation tests"
```

### Task 5: Phase-4b validation and coverage artifact

**Files:**
- Artifact: `coverage.phase4b.out` (generated)

- [ ] **Step 1: Run required command**

Run: `go test ./core/engine -run TestAskHybrid -v`
Expected: PASS.

- [ ] **Step 2: Run full engine suite**

Run: `go test ./core/engine -v`
Expected: PASS.

- [ ] **Step 3: Run covered-module regression suite**

Run: `go test ./core/utils ./core/retrieval ./core/index ./core/engine`
Expected: PASS.

- [ ] **Step 4: Run full repository regression (non-blocking)**

Run: `go test ./...`
Expected: Prefer PASS; document unrelated failures if any.

- [ ] **Step 5: Generate required phase-4b coverage artifact**

Run: `go test -coverprofile=coverage.phase4b.out ./core/engine`
Expected: PASS and file generated.

- [ ] **Step 6: Verify artifact exists**

Run: `test -f coverage.phase4b.out`
Expected: exit code 0.

- [ ] **Step 7: Artifact commit policy**

```bash
# If user requests artifact commit:
git add coverage.phase4b.out
git commit -m "chore(test): capture phase-4b ask-hybrid coverage baseline"

# Otherwise keep untracked and report success
```

## Acceptance Checklist

- [ ] `TestAskHybrid_*` suite added with deterministic fixtures.
- [ ] Hybrid success, prompt contract, source inclusion, empty fallback, and error propagation cases covered.
- [ ] Message contract validated (2 messages, role order, non-empty system, user prompt fragments).
- [ ] Required command passes: `go test ./core/engine -run TestAskHybrid -v`.
- [ ] `coverage.phase4b.out` generated locally.

## Notes

- Keep hybrid Ask tests integration-focused; avoid duplicating Phase-3 ranking internals.
- Prefer fragment assertions over brittle full prompt string assertions.
