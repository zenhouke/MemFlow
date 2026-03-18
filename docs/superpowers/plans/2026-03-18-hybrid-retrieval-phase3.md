# Hybrid Retrieval Phase-3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add deterministic hybrid retrieval behavior tests for engine search, covering intent-driven depth selection, fusion effects, fallback paths, and observable output guarantees.

**Architecture:** Reuse Phase-2 engine test harness and add a dedicated `hybrid_search_test.go` suite with deterministic fixtures. Extend test fakes with a call-counted LLM fake to validate analyzer branching. Keep assertions black-box oriented (returned IDs/order/count/timestamps), with limited branch checks through fake call counters and known fixture outcomes.

**Tech Stack:** Go (`testing`), MemFlow `core/engine`, `core/retrieval`, `core/index`

---

## File Structure Map

- Create: `core/engine/hybrid_search_test.go`
- Modify: `core/engine/test_fakes_test.go` (add fake LLM client)
- Modify (only if necessary for deterministic assertions): `core/engine/search.go`

### Task 1: Add hybrid test fake dependencies

**Files:**
- Modify: `core/engine/test_fakes_test.go`

- [ ] **Step 1: Add fake LLM client with deterministic response and call counter**

```go
type fakeLLMClient struct {
    response string
    err error
    chatCallCount int
}

func (f *fakeLLMClient) Chat(ctx context.Context, messages []llm.Message) (string, error) {
    f.chatCallCount++
    if f.err != nil { return "", f.err }
    return f.response, nil
}
```

- [ ] **Step 2: Add helper constructor for hybrid-enabled test engine config**

```go
func newHybridTestEngine(embed *fakeEmbedder, now time.Time) *MemoryEngine {
    // EnableHybridSearch=true, explicit HybridSearchConfig values
}
```

- [ ] **Step 3: Run sanity test command**

Run: `go test ./core/engine -run TestHybrid -v`
Expected: compiles (may show no tests yet if Task 2 not started).

- [ ] **Step 4: Commit**

```bash
git add core/engine/test_fakes_test.go
git commit -m "test(engine): add fake llm and hybrid test helpers"
```

### Task 2: Add hybrid path routing and error behavior tests

**Files:**
- Create: `core/engine/hybrid_search_test.go`

- [ ] **Step 1: Add `TestHybrid_Search_EmbedderError`**

```go
func TestHybrid_Search_EmbedderError(t *testing.T) {}
```

- [ ] **Step 2: Add `TestHybrid_Search_UsesHybridPath_ConfigEnabled`**

```go
func TestHybrid_Search_UsesHybridPath_ConfigEnabled(t *testing.T) {
    // configure engine hybrid=true with explicit fusion weights
    // fixture where lexical+symbolic should elevate doc-B over semantically-close doc-A
    // assert deterministic expected top result/order for hybrid-enabled settings
}
```

- [ ] **Step 3: Run focused tests**

Run: `go test ./core/engine -run 'TestHybrid_Search_(EmbedderError|UsesHybridPath_ConfigEnabled)$' -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add core/engine/hybrid_search_test.go
git commit -m "test(engine): add hybrid path routing and error tests"
```

### Task 3: Add normative `k` selection tests (adaptive/non-adaptive)

**Files:**
- Modify: `core/engine/hybrid_search_test.go`

- [ ] **Step 1: Add adaptive lower/upper clamp cases from spec decision table**

```go
func TestHybrid_KSelection_AdaptiveClamp(t *testing.T) {
    // case 1 expected k=5
    // case 2 expected k=8
}
```

- [ ] **Step 2: Add adaptive delta=0 default behavior case**

```go
func TestHybrid_KSelection_AdaptiveDeltaZeroUsesDefault(t *testing.T) {
    // expected k=10
}
```

- [ ] **Step 3: Add non-adaptive retrieval-depth case**

```go
func TestHybrid_KSelection_NonAdaptiveUsesIntentDepth(t *testing.T) {
    // expected k=7
}
```

- [ ] **Step 4: Run focused tests**

Run: `go test ./core/engine -run 'TestHybrid_KSelection_' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add core/engine/hybrid_search_test.go
git commit -m "test(engine): add hybrid retrieval-depth selection tests"
```

### Task 4: Add intent branching and fallback behavior tests

**Files:**
- Modify: `core/engine/hybrid_search_test.go`

- [ ] **Step 1: Add short-query fast-path boundary tests with call-count assertions**

```go
func TestHybrid_Intent_ShortQueryFastPath_OneAndTwoTokens_NoLLMCall(t *testing.T) {
    // 1-token and 2-token queries => chatCallCount == 0
}

func TestHybrid_Intent_ThreeTokens_NotFastPath(t *testing.T) {
    // 3-token query => fast path not applied; LLM path eligible (call count behavior differs)
}
```

- [ ] **Step 2: Add LLM failure fallback test**

```go
func TestHybrid_Intent_LLMFailure_FallsBackRuleBased(t *testing.T) {}
```

- [ ] **Step 3: Add LLM success intent parsing test**

```go
func TestHybrid_Intent_LLMSuccess_UsesIntentFields(t *testing.T) {}
```

- [ ] **Step 4: Run focused tests**

Run: `go test ./core/engine -run 'TestHybrid_Intent_' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add core/engine/hybrid_search_test.go
git commit -m "test(engine): add hybrid intent branch and fallback tests"
```

### Task 5: Add fusion channel effect and output guarantee tests

**Files:**
- Modify: `core/engine/hybrid_search_test.go`

- [ ] **Step 1: Add semantic-dominant ranking fixture**

```go
func TestHybrid_Fusion_SemanticDominantOrdering(t *testing.T) {}
```

- [ ] **Step 2: Add lexical-dominant ranking fixture**

```go
func TestHybrid_Fusion_LexicalDominantOrdering(t *testing.T) {}
```

- [ ] **Step 3: Add symbolic-constraint ranking fixture**

```go
func TestHybrid_Fusion_SymbolicConstraintBoost(t *testing.T) {}
```

- [ ] **Step 4: Add symbolic baseline (no constraints) fixture**

```go
func TestHybrid_Fusion_SymbolicBaseline_NoConstraints(t *testing.T) {}
```

- [ ] **Step 5: Add output guarantees test**

```go
func TestHybrid_Output_CountOrderingAndLastAccessedAt(t *testing.T) {
    // count <= k
    // deterministic non-tied ordering checks
    // returned LastAccessedAt and persisted in-memory LastAccessedAt via Get
}
```

- [ ] **Step 6: Run focused tests**

Run: `go test ./core/engine -run 'TestHybrid_(Fusion|Output)_' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add core/engine/hybrid_search_test.go
git commit -m "test(engine): add hybrid fusion behavior and output guarantees"
```

### Task 6: Phase-3 validation and coverage artifact

**Files:**
- Artifact: `coverage.phase3.out` (generated)

- [ ] **Step 1: Run required phase-3 command**

Run: `go test ./core/engine -run TestHybrid -v`
Expected: PASS.

- [ ] **Step 2: Run full engine tests**

Run: `go test ./core/engine -v`
Expected: PASS.

- [ ] **Step 3: Run broader regression checks**

Run: `go test ./core/utils ./core/retrieval ./core/index ./core/engine`
Expected: PASS.

- [ ] **Step 4: Run full repository check (non-blocking)**

Run: `go test ./...`
Expected: Prefer PASS; document unrelated failures if any.

- [ ] **Step 5: Generate and verify phase-3 coverage artifact**

Run: `go test -coverprofile=coverage.phase3.out ./core/engine && test -f coverage.phase3.out`
Expected: artifact generated and verified.

- [ ] **Step 6: Artifact commit policy**

```bash
# If user requests commit:
git add coverage.phase3.out
git commit -m "chore(test): capture phase-3 hybrid coverage baseline"

# If user does not request commit:
# keep untracked and report generation success
```

## Acceptance Checklist

- [ ] Hybrid tests use `TestHybrid*` naming convention.
- [ ] Adaptive/non-adaptive `k` behavior verified against spec decision table.
- [ ] Short-query fast path and LLM fallback behavior validated.
- [ ] Fusion channel effect tests (semantic/lexical/symbolic) pass with deterministic fixtures.
- [ ] Output guarantees validated (count/order/timestamp + persisted in-memory timestamp update).
- [ ] Required command passes: `go test ./core/engine -run TestHybrid -v`.
- [ ] `coverage.phase3.out` generated locally.

## Notes

- Keep fixture scores clearly separated for ordering assertions.
- Avoid strict assertions on tied-score order.
- Prefer behavior-level expectations over internal field coupling.
