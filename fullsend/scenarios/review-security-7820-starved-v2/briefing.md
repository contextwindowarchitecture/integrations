# review-security-7820-starved-v2: the assembled briefing

Profile `fullsend-review-security-briefing` v2, snapshot `e121254a41f69eea771734460fad917b6be129442f6891bdc33c6654548b2149`.

**Refused: `evidence_required`, recovery `precompute_summary`.** No sandbox starts and no model is called. The assembler asks for shorter forms computed ahead of time, such as a review split per file, then a new snapshot. fullsend maps the refusal to an outcome instead: see [refusals](../../README.md#46-refusals-become-fullsend-outcomes-with-no-model-tokens-spent).

## Left out, and why

| Item | Slot | Decided by | Reason |
| --- | --- | --- | --- |
| `run:review/18990#F2` | - | its producer | `revoked` |
| `run:review/18990#F3` | `interaction.memory` | the assembler | `conflict_lost` |
| `ci:unit-tests@9f1e77aa` | `evidence.tool_results` | the assembler | `superseded`, keeps `ci:unit-tests@abc1230f` |
| `state:requester` | `state.user` | the assembler | `over_budget` |
| `diff:docs/guides/user/runtime-flags.md@abc1230f` | `evidence.tool_results` | the assembler | `over_budget` |
| `diff:internal/runtime/claude_test.go@abc1230f` | `evidence.tool_results` | the assembler | `over_budget` |
| `ci:unit-tests@abc1230f` | `evidence.tool_results` | the assembler | `over_budget` |
| `diff:internal/runtime/claude.go@abc1230f` | `evidence.tool_results` | the assembler | `over_budget` |

## Declared conflicts

The run declares these from structured data before assembly; the assembler never looks for contradictions in text. It decides each one before fitting the budget.

| Group | Kind | Outcome | Winner |
| --- | --- | --- | --- |
| `fact:ci.unit-tests.status` | fact | resolved by policy | `ci:unit-tests@abc1230f` |

A winner can still be left out afterwards for budget, as here: winning a conflict decides which claim is current, not that it fits.

[trace.json](trace.json) holds every decision, including each included item's token count; [snapshot.json](snapshot.json) is the frozen input it was assembled from.
