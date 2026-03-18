# Ask Phase-4 (Simple Mode) Test Design

## Context and Goal

Phase-4 targets `MemoryEngine.Ask` behavior with retrieval in simple mode (`EnableHybridSearch=false`).

Goal: establish deterministic coverage for Ask’s control flow, prompt assembly, and LLM interaction contract without introducing production behavior changes.

## Scope

In scope:

- `Ask` behavior under simple search mode only
- Error propagation and no-result fallback behavior
- Prompt construction validation (key fragments)
- LLM invocation behavior and response passthrough

Out of scope (for this step):

- Ask behavior under hybrid search mode (scheduled as next extension)
- LLM output quality/content semantics
- External vector store integration

## Approach

Behavior-first testing with deterministic fixtures:

- Keep production code unchanged unless required for testability.
- Reuse existing engine test harness (`fakeEmbedder`, fixed `nowFn`).
- Extend fake LLM client with message-capture capability for prompt assertions.

## Test Architecture

### Files

- Create: `core/engine/ask_test.go`
- Modify: `core/engine/test_fakes_test.go` (capture `lastMessages` in fake LLM)

### Design Principles

- Use table-driven tests where practical.
- Assert externally observable behavior first.
- For prompt text, assert required fragments via `strings.Contains` instead of full-string equality.
- Keep fixtures deterministic and tie-free for retrieval order assumptions.

## Behavior Coverage Matrix

### 1) Preconditions and error paths

- `Ask` with `llmClient=nil` returns error: `LLM client not set`.
- `Ask` returns `Search` errors unchanged (e.g., embedder failure).
- When `Search` errors, LLM `Chat` is not called.

### 2) Empty-result path

- If `Search` returns zero memories, `Ask` returns exact fallback text:
  - `No relevant memories found.`
- LLM `Chat` is not called in this path.

### 3) Success path and prompt contract

- On successful retrieval with one or more memories:
  - `Ask` calls LLM exactly once.
  - Sends two messages:
    - system: assistant instruction message
    - user: assembled prompt with memory context and question
- User prompt must include these fragments:
  - `Memory Context:`
  - numbered context lines like `[1] ...`
  - optional source line when `OriginalContent` exists: `Source: ...`
  - `Question: <question>`
  - trailing `Answer:` section

### 4) Response passthrough

- LLM response string is returned unchanged by `Ask`.

## Determinism and Stability

- Use fixed `nowFn` and deterministic embeddings.
- Disable hybrid search explicitly in all Ask Phase-4 tests.
- Use stable fixtures to avoid ranking ambiguity in context ordering.

## Validation Commands

Required:

- `go test ./core/engine -run TestAsk -v`

Recommended:

- `go test ./core/engine -v`
- `go test ./core/utils ./core/retrieval ./core/index ./core/engine`
- `go test ./...`

Coverage artifact:

- `go test -coverprofile=coverage.phase4.out ./core/engine`

## Success Criteria

- Ask tests added for precondition, error, empty-result, and success paths.
- Prompt contract verified via fragment assertions.
- Required command passes: `go test ./core/engine -run TestAsk -v`.
- No regressions in broader engine and covered-module test suites.
- `coverage.phase4.out` generated locally.

## Risks and Mitigations

- Risk: brittle prompt assertions.
  - Mitigation: fragment-based checks for contract-critical text only.

- Risk: hidden coupling with retrieval order.
  - Mitigation: deterministic fixtures and minimal order assumptions.

- Risk: accidental hybrid-path interference.
  - Mitigation: explicitly set `EnableHybridSearch=false` in Ask tests.

## Next Step

After this simple-mode Ask coverage:

- Extend Ask tests to hybrid mode using the same prompt/interaction contract with hybrid retrieval inputs.
