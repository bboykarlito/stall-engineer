# ADR-001: Explicit engineering stages with Pi execution

## Status

Partially implemented. Behaviour contracts, Pi RPC execution, and a local-only CLI using libstall.Runner are implemented and tested with fake harnesses/processes and temporary repositories. Budget policy, enforced test commands, publication, and live Pi compatibility remain unimplemented or unverified.

## Context

This project is libstall's first concrete consumer. The desired workflow accepts an engineering task, implements it, reviews the result, and pushes it to Git. libstall currently supplies interfaces and data types, not runner execution or budget enforcement.

## Direction

- Keep sequencing in Go application code, not in a workflow graph or a single all-purpose agent prompt.
- Use separate implementation and review behaviours and isolated Pi conversational sessions.
- Pass the original task, implementation summary, and actual change evidence explicitly into review. A new conversation does not mean a new filesystem: both stages inspect the same proposed changes.
- Keep the Pi adapter local to this application initially, implementing `stall.Harness`. Extract it only when its contract has been exercised.
- Use Pi RPC over stdin/stdout for subprocess control. Wait for `agent_settled`, not merely prompt acceptance or `agent_end`; inspect final message errors and obtain session statistics before teardown.
- Explicitly configure provider/model, working directory, tools, and session persistence. A new process with `--no-session` per execution prevents conversational reuse.
- Disable discovered extensions, skills, prompt templates, and context files for the initial controlled adapter, and reject project-resource trust. Supply necessary repository conventions deliberately. This avoids ambient behaviour, but is not a sandbox.
- Use only `read,grep,find,ls` for review, with no shell or extension tools. Tool configuration must be enforced by Pi rather than relying on instructions.
- Treat commit and push as deterministic application operations, not another LLM stage.

## Local CLI increment

The CLI requires an explicit repository and task. Both model stages use the selected provider/model with independent sessions and deadlines. Repository preflight requires a clean worktree and index on a named non-protected branch. Protected branches are main/master/trunk, a caller-configured base branch, and the locally recorded origin default when available; no remote lookup is performed.

Implementation is followed by binary-capable Git diff capture against the original HEAD, including non-ignored untracked files. Submodule repositories are rejected because this first version cannot provide complete nested change evidence. External Git diff/text conversion helpers are disabled. Evidence is capped at 4 MiB and passed explicitly as JSON alongside the task and summary to the read-only reviewer. After approval, the application rechecks HEAD, branch, and the captured working-tree changes.

No test execution is enforced by deterministic application code yet; implementation is prompted to run checks and review is expected to question insufficient evidence. Ignored files and index-only differences with unchanged working-tree contents are outside the captured patch. The snapshot check is not a lock against concurrent writers. The CLI never commits, pushes, resets, or rolls back; failure preserves edits and available stage results for inspection.

## Publication policy to implement

Start from a clean, explicitly selected repository and a dedicated feature branch. Capture and review the complete proposed change set, including new files. Run configured checks, require valid review approval, and verify the reviewed state has not changed before committing or pushing. Do not force-push or publish to a default branch. Require explicit push opt-in and an explicit destination.

The implementation prompt asks the model not to commit or push. If implementation has unrestricted shell access, that instruction cannot enforce a publication boundary. Before claiming that publication is gated, choose a real isolation strategy (for example, an implementation environment without Git credentials or network access, with publication outside it). A disposable worktree alone does not isolate credentials, filesystem access, or networking. Never advertise a prompt-only restriction as enforcement.

## Cost and failure semantics

No automatic application retries. Budget accounting starts fresh for each stage, but no hard cap is promised until the adapter can actually enforce one. Pi's own retries and compaction also need an explicit policy, and their usage must not disappear from totals. Preserve available output and usage on failure and stop the workflow when preparation, execution, checks, or review fails.

## Prototype limitations

The adapter currently disables Pi retry and compaction and gathers token totals after settlement. It does not enforce a spend cap. Usage availability and monetary cost remain unresolved in libstall: failures before statistics retrieval return zero counts that must be treated as unknown, not zero spend. This is an explicit deviation from the original plan to resolve those types before implementing the adapter; budget-dependent admission and publication must not rely on this prototype's accounting.

Cancellation closes pipes and terminates/reaps the Pi process, without promising termination of all shell descendants. A context deadline is required operationally to bound unresponsive RPC peers. Unexpected interactive UI requests fail rather than waiting indefinitely for input.

## Alternatives

- One Pi session implementing, reviewing, and pushing: simpler wiring, but implicit state and no independent review or application-controlled publication gate.
- Pi's TypeScript SDK: useful for Node applications; RPC avoids introducing a Node bridge solely to host a Go adapter.
- LLM-controlled Git publication: unnecessary nondeterminism for operations that Go can perform explicitly and test locally.

## Open questions

- Which Pi provider ID should be selected explicitly? Both stages initially default to `gpt-5.6-terra` (user-selected); availability remains unverified. Provider/model overrides are supported by configuration.
- Which repository and test commands will be the first target?
- What isolation will enforce the implementation/publication boundary?
- Which Git remote and feature-branch naming policy should the CLI use?
- How should libstall represent unavailable usage, cached tokens, monetary cost, and enforceable budgets?

## References

- [Pi RPC documentation](https://github.com/earendil-works/pi-mono/blob/main/packages/coding-agent/docs/rpc.md)
- [Pi CLI documentation](https://github.com/earendil-works/pi-mono/blob/main/packages/coding-agent/README.md)
- [libstall execution model](../../../libstall/docs/decisions/001-execution-model.md)
