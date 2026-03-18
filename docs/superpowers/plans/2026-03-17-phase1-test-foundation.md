# MemFlow Phase-1 Test Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add deterministic Phase-1 unit tests for core utility, retrieval intent, and index modules so regressions are caught early without external dependencies.

**Architecture:** Add colocated `_test.go` files in `core/utils`, `core/retrieval`, and `core/index` using table-driven tests and focused subtests. Keep all tests in-process with a fake `llm.LLMClient` for intent analysis behavior checks. Enforce deterministic assertions (epsilon for float, set/sort normalization for unordered results).

**Tech Stack:** Go (`testing`), existing MemFlow packages (`core/utils`, `core/retrieval`, `core/index`, `core/llm`)

---

## File Structure Map

- Create: `core/utils/common_test.go` (tests for `ToFloat32`, `Max`, `Min`)
- Create: `core/utils/string_test.go` (tests for `UniqueStrings`)
- Create: `core/utils/vector_test.go` (tests for `Cosine`, `ExpDecay`)
- Create: `core/retrieval/intent_test.go` (tests for analyzer/fallback parsing/helpers)
- Create: `core/index/metadata_test.go` (tests for metadata add/search/delete/time)
- Create: `core/index/bm25_test.go` (tests for tokenizer/add/delete/search/topK)

### Task 1: Add tests for `core/utils/common.go`

**Files:**
- Create: `core/utils/common_test.go`
- Test: `core/utils/common_test.go`

- [ ] **Step 1: Write failing tests for `ToFloat32`**

```go
func TestToFloat32(t *testing.T) {
    tests := []struct {
        name string
        in   []float64
        want []float32
    }{
        {name: "empty", in: []float64{}, want: []float32{}},
        {name: "normal", in: []float64{1.5, -2.25, 0}, want: []float32{1.5, -2.25, 0}},
    }
    // loop + exact slice checks
}
```

- [ ] **Step 2: Run target test to validate baseline behavior**

Run: `go test ./core/utils -run TestToFloat32 -v`
Expected: captures current behavior; if failing, adjust tests/implementation, then rerun to PASS.

- [ ] **Step 3: Complete minimal assertions for `ToFloat32`**

```go
if len(got) != len(tt.want) { t.Fatalf("len: got %d want %d", len(got), len(tt.want)) }
for i := range got {
    if got[i] != tt.want[i] { t.Fatalf("idx %d: got %v want %v", i, got[i], tt.want[i]) }
}
```

- [ ] **Step 4: Add tests for `Max` and `Min` branches**

```go
func TestMax(t *testing.T) { /* greater/less/equal */ }
func TestMin(t *testing.T) { /* greater/less/equal */ }
```

- [ ] **Step 5: Run package tests**

Run: `go test ./core/utils -run 'Test(ToFloat32|Max|Min)$' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add core/utils/common_test.go
git commit -m "test(utils): add common function unit tests"
```

### Task 2: Add tests for `core/utils/string.go` and `core/utils/vector.go`

**Files:**
- Create: `core/utils/string_test.go`
- Create: `core/utils/vector_test.go`
- Test: `core/utils/string_test.go`
- Test: `core/utils/vector_test.go`

- [ ] **Step 1: Write failing tests for `UniqueStrings` behavior**

```go
func TestUniqueStrings(t *testing.T) {
    // dedupe + order + drop empty string cases
}
```

- [ ] **Step 2: Run `UniqueStrings` test to validate baseline behavior**

Run: `go test ./core/utils -run TestUniqueStrings -v`
Expected: captures current behavior; if failing, adjust tests/implementation, then rerun to PASS.

- [ ] **Step 3: Implement minimal assertions for `UniqueStrings`**

```go
// compare length and element-by-element order
```

- [ ] **Step 4: Write failing tests for `Cosine` and `ExpDecay`**

```go
func TestCosine(t *testing.T) {
    // mismatch length => 0
    // empty => 0
    // zero norm => 0
    // identical ~= 1
    // orthogonal ~= 0
}

func TestExpDecay(t *testing.T) {
    // t=0 => 1
    // lambda=0 => 1
    // regular decay with epsilon check
}
```

- [ ] **Step 5: Add epsilon helper and complete assertions**

```go
const eps = 1e-9
func almostEqual(a, b float64) bool { return math.Abs(a-b) <= eps }
```

- [ ] **Step 6: Run utils package tests**

Run: `go test ./core/utils -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add core/utils/string_test.go core/utils/vector_test.go
git commit -m "test(utils): cover string dedupe and vector math"
```

### Task 3: Add tests for `core/retrieval/intent.go`

**Files:**
- Create: `core/retrieval/intent_test.go`
- Test: `core/retrieval/intent_test.go`

- [ ] **Step 1: Add test fake for `llm.LLMClient`**

```go
type fakeLLMClient struct {
    response      string
    err           error
    chatCallCount int
}

func (f *fakeLLMClient) Chat(ctx context.Context, messages []llm.Message) (string, error) {
    f.chatCallCount++
    return f.response, f.err
}
```

- [ ] **Step 2: Write tests for `NewQueryAnalyzer` and `Analyze` paths**

```go
func TestNewQueryAnalyzer_DefaultBaseK(t *testing.T) { /* baseK<=0 => 5 */ }
func TestAnalyze_NoLLM_UsesRuleBased(t *testing.T) { /* nil client */ }
func TestAnalyze_ShortQuery_DoesNotCallLLM(t *testing.T) { /* <=2 words */ }
func TestAnalyze_NonShortQuery_CallsLLMOnce(t *testing.T) { /* chatCallCount == 1 */ }
func TestAnalyze_LLMError_FallsBackToRuleBased(t *testing.T) { /* fallback + call count */ }
```

- [ ] **Step 3: Run focused tests to validate baseline behavior**

Run: `go test ./core/retrieval -run 'Test(NewQueryAnalyzer_DefaultBaseK|Analyze_.*)' -v`
Expected: captures current behavior; if failing, adjust tests/implementation, then rerun to PASS.

- [ ] **Step 4: Add parse-response tests and helper tests**

```go
func TestParseIntentResponse_ValidJSON(t *testing.T) {}
func TestParseIntentResponse_EmbeddedJSON(t *testing.T) {}
func TestParseIntentResponse_InvalidJSON_FallsBack(t *testing.T) {}
func TestParseIntentResponse_ZeroRetrievalDepth_UsesBaseK(t *testing.T) {}
func TestCalculateDynamicK(t *testing.T) {}
```

- [ ] **Step 5: Add branch tests for `ruleBasedAnalyze` and keyword/entity extraction**

```go
func TestRuleBasedAnalyze_Branches(t *testing.T) {}
func TestExtractKeywords(t *testing.T) {}
func TestExtractEntities(t *testing.T) {}
func TestContainsTimeKeywords(t *testing.T) {}
func TestContainsAggregationKeywords(t *testing.T) {}
```

- [ ] **Step 6: Run retrieval package tests**

Run: `go test ./core/retrieval -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add core/retrieval/intent_test.go
git commit -m "test(retrieval): add query intent analyzer unit coverage"
```

### Task 4: Add tests for `core/index/metadata.go`

**Files:**
- Create: `core/index/metadata_test.go`
- Test: `core/index/metadata_test.go`

- [ ] **Step 1: Write tests for add/search entity and topic normalization**

```go
func TestMetadataIndex_AddAndDirectSearch(t *testing.T) {
    // SearchByEntity and SearchByTopic should be case-insensitive
}
```

- [ ] **Step 2: Run focused metadata tests to validate baseline behavior**

Run: `go test ./core/index -run TestMetadataIndex_AddAndDirectSearch -v`
Expected: captures current behavior; if failing, adjust tests/implementation, then rerun to PASS.

- [ ] **Step 3: Add tests for composite `Search` filters and no-filter path**

```go
func TestMetadataIndex_Search_ByFilters(t *testing.T) {}
// include entity-only, topic-only, tag-only, mixed (entity+tag/topic+tag), and no-match tag cases
func TestMetadataIndex_Search_NoFilterReturnsAllWithinTimeRange(t *testing.T) {}
```

- [ ] **Step 4: Add tests for delete and metadata lookup**

```go
func TestMetadataIndex_Delete_RemovesAllIndexes(t *testing.T) {}
func TestMetadataIndex_GetMetadata(t *testing.T) {}
```

- [ ] **Step 5: Add time range boundary test**

```go
func TestMetadataIndex_SearchByTimeRange_InclusiveBoundaries(t *testing.T) {}
```

- [ ] **Step 6: Normalize unordered comparisons in helpers**

```go
func asSet(items []string) map[string]bool { /* set compare for map-order safety */ }
```

- [ ] **Step 7: Run index metadata tests**

Run: `go test ./core/index -run TestMetadataIndex -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add core/index/metadata_test.go
git commit -m "test(index): add metadata index behavior tests"
```

### Task 5: Add tests for `core/index/bm25.go`

**Files:**
- Create: `core/index/bm25_test.go`
- Test: `core/index/bm25_test.go`

- [ ] **Step 1: Write tests for tokenizer normalization**

```go
func TestBM25Index_tokenize(t *testing.T) {
    // punctuation/case/stop-word filtering
}
```

- [ ] **Step 2: Run tokenizer test to validate baseline behavior**

Run: `go test ./core/index -run TestBM25Index_tokenize -v`
Expected: captures current behavior; if failing, adjust tests/implementation, then rerun to PASS.

- [ ] **Step 3: Add add/delete state transition tests**

```go
func TestBM25Index_AddDelete_UpdatesStats(t *testing.T) {
    // docCount, avgDocLen, docFreq transitions
}
```

- [ ] **Step 4: Add search behavior tests**

```go
func TestBM25Index_Search_EmptyIndex(t *testing.T) {}
func TestBM25Index_Search_TopKAndSortedScores(t *testing.T) {}
func TestBM25Index_Search_ReturnsPositiveScoresOnly(t *testing.T) {}
func TestBM25Index_Search_DeletedDocNotReturned(t *testing.T) {}
```

- [ ] **Step 5: Handle tie-sensitive assertions safely**

```go
// assert descending score when scores differ; use set membership for equal-score groups
```

- [ ] **Step 6: Run index package tests**

Run: `go test ./core/index -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add core/index/bm25_test.go
git commit -m "test(index): add BM25 tokenizer and search tests"
```

### Task 6: Phase-1 validation and coverage artifact

**Files:**
- Modify: none
- Artifact: `coverage.phase1.out` (generated)

- [ ] **Step 1: Run required scoped Phase-1 tests**

Run: `go test ./core/utils ./core/retrieval ./core/index`
Expected: PASS.

- [ ] **Step 2: Run full repository tests (non-blocking signal)**

Run: `go test ./...`
Expected: Prefer PASS; if FAIL outside scope, record failures without blocking Phase-1 completion.

- [ ] **Step 3: Generate baseline coverage artifact**

Run: `go test -coverprofile=coverage.phase1.out ./core/utils ./core/retrieval ./core/index`
Expected: `coverage.phase1.out` created at repo root.

- [ ] **Step 4: Verify artifact exists**

Run: `test -f coverage.phase1.out`
Expected: exit code 0 (artifact exists).

- [ ] **Step 5: Commit generated artifact decision**

```bash
# If repo policy allows coverage artifact commits:
git add coverage.phase1.out
git commit -m "chore(test): capture phase-1 baseline coverage"

# If repo policy does not allow generated artifacts:
# skip commit and report command/output in PR description
```

## Acceptance Checklist

- [ ] Tests exist for all in-scope source files listed in Phase-1 spec.
- [ ] Required command passes: `go test ./core/utils ./core/retrieval ./core/index`
- [ ] Critical branches validated:
  - [ ] `Cosine` early returns (mismatch/empty/zero norm)
  - [ ] `Analyze` paths (nil LLM, short query fast path, LLM error fallback)
  - [ ] `parseIntentResponse` paths (raw/embedded/invalid JSON + retrieval depth fallback)
  - [ ] `MetadataIndex.Search` filter path + no-filter path + time filter
  - [ ] `BM25Index.Search` empty index + positive-score-only + topK truncation
- [ ] Coverage artifact generated at `coverage.phase1.out`.

## Notes for Implementers

- Keep tests behavior-driven and avoid private-state coupling unless state is part of observed contract.
- Prefer set comparison for unordered outputs from map-backed structures.
- Keep commits small and frequent (one task group per commit).
