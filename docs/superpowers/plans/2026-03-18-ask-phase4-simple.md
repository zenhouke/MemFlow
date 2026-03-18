# Ask Phase-4 (Simple Mode) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add deterministic tests for `MemoryEngine.Ask` in simple search mode, covering precondition, error propagation, empty-result fallback, prompt contract, and LLM response passthrough.

**Architecture:** Reuse existing engine fakes and deterministic setup. Extend fake LLM with last-message capture. Add `ask_test.go` with behavior-focused tests that assert return values/errors/call counts and prompt fragments via contains-checks.

**Tech Stack:** Go (`testing`), MemFlow `core/engine`, `core/llm`

---

## File Structure Map

- Modify: `core/engine/test_fakes_test.go`
- Create: `core/engine/ask_test.go`

### Task 1: Add LLM message capture support in test fake

**Files:**
- Modify: `core/engine/test_fakes_test.go`

- [ ] **Step 1: Extend fake LLM with message capture field**

```go
type fakeLLMClient struct {
    response string
    err error
    chatCallCount int
    lastMessages []llm.Message
}
```

- [ ] **Step 2: Capture call messages in `Chat`**

```go
func (f *fakeLLMClient) Chat(ctx context.Context, messages []llm.Message) (string, error) {
    f.chatCallCount++
    f.lastMessages = append([]llm.Message(nil), messages...)
    ...
}
```

- [ ] **Step 3: Run sanity check**

Run: `go test ./core/engine -run TestAsk -v`
Expected: compile pass (possibly no tests yet).

- [ ] **Step 4: Commit**

```bash
git add core/engine/test_fakes_test.go
git commit -m "test(engine): capture llm messages in test fake"
```

### Task 2: Add Ask precondition and error-path tests

**Files:**
- Create: `core/engine/ask_test.go`

- [ ] **Step 1: Add nil-LLM precondition test**

```go
func TestAsk_NoLLMClient_ReturnsError(t *testing.T) {
    // assert err message exactly equals "LLM client not set"
}
```

- [ ] **Step 2: Add Search-error propagation test**

```go
func TestAsk_SearchError_Propagates(t *testing.T) {
    // embedder error via Search path
    // assert returned error matches original embedder error
    // ensure llm chat count == 0
}
```

- [ ] **Step 3: Run focused tests**

Run: `go test ./core/engine -run 'TestAsk_(NoLLMClient_ReturnsError|SearchError_Propagates)$' -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add core/engine/ask_test.go
git commit -m "test(engine): add ask precondition and error-path tests"
```

### Task 3: Add Ask empty-result and success-path tests

**Files:**
- Modify: `core/engine/ask_test.go`

- [ ] **Step 1: Add empty-result fallback test**

```go
func TestAsk_NoRelevantMemories_ReturnsFallback(t *testing.T) {
    // expect exact fallback text: "No relevant memories found."
    // llm chat count == 0
}
```

- [ ] **Step 2: Add success response passthrough test**

```go
func TestAsk_Success_ReturnsLLMAnswer(t *testing.T) {
    // llm response should pass through unchanged
    // llm called exactly once
}
```

- [ ] **Step 3: Run focused tests**

Run: `go test ./core/engine -run 'TestAsk_(NoRelevantMemories_ReturnsFallback|Success_ReturnsLLMAnswer)$' -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add core/engine/ask_test.go
git commit -m "test(engine): add ask empty-result and success tests"
```

### Task 4: Add Ask prompt contract tests

**Files:**
- Modify: `core/engine/ask_test.go`

- [ ] **Step 1: Add prompt-structure test with key fragments**

```go
func TestAsk_PromptContainsRequiredFragments(t *testing.T) {
    // deterministic two-memory fixture to ensure [1] and [2] context lines exist
    // assert Memory Context, [1], [2], Question, Answer
}
```

- [ ] **Step 2: Add source-line inclusion test for OriginalContent**

```go
func TestAsk_PromptIncludesSourceWhenOriginalContentPresent(t *testing.T) {}
```

- [ ] **Step 3: Validate captured message contract**

```go
// exactly 2 messages: system + user
// system role/content sanity check
// user content fragment checks with strings.Contains
```

- [ ] **Step 3.1: Enforce simple-mode setup helper**

```go
func newAskSimpleTestEngine(...) *MemoryEngine {
    eng := newTestEngineWithNow(...)
    eng.config.EnableHybridSearch = false
    return eng
}

// require all TestAsk_* cases to use this helper
```

- [ ] **Step 4: Run focused tests**

Run: `go test ./core/engine -run 'TestAsk_(PromptContainsRequiredFragments|PromptIncludesSourceWhenOriginalContentPresent)$' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add core/engine/ask_test.go
git commit -m "test(engine): add ask prompt contract tests"
```

### Task 5: Phase-4 validation and coverage artifact

**Files:**
- Artifact: `coverage.phase4.out` (generated)

- [ ] **Step 1: Run required phase-4 command**

Run: `go test ./core/engine -run TestAsk -v`
Expected: PASS.

- [ ] **Step 2: Run full engine tests**

Run: `go test ./core/engine -v`
Expected: PASS.

- [ ] **Step 3: Run broader regression check**

Run: `go test ./core/utils ./core/retrieval ./core/index ./core/engine`
Expected: PASS.

- [ ] **Step 4: Run full repository check (non-blocking)**

Run: `go test ./...`
Expected: Prefer PASS; document unrelated failures if any.

- [ ] **Step 5: Generate and verify coverage artifact**

Run: `go test -coverprofile=coverage.phase4.out ./core/engine && test -f coverage.phase4.out`
Expected: file generated and verified.

- [ ] **Step 6: Artifact commit policy**

```bash
# If user requests commit:
git add coverage.phase4.out
git commit -m "chore(test): capture phase-4 ask coverage baseline"

# If user does not request commit:
# keep untracked and report success
```

## Acceptance Checklist

- [ ] Ask precondition/error/empty/success paths covered in simple mode.
- [ ] Prompt contract assertions use fragment checks, not full-string matching.
- [ ] LLM call count and message capture behavior validated.
- [ ] Required command passes: `go test ./core/engine -run TestAsk -v`.
- [ ] `coverage.phase4.out` generated locally.

## Notes

- Force `EnableHybridSearch=false` in Ask tests to keep phase scope strict.
- Use deterministic embeddings and fixed timestamps for stable retrieval context ordering.
