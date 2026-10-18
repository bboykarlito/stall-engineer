# ADR-001: Explicit engineering stages with Pi execution

## Status

Proposed. Behaviour contracts are implemented; execution and publication policies below remain to be implemented and validated.

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

## Publication policy to implement

Start from a clean, explicitly selected repository and a dedicated feature branch. Capture and review the complete proposed change set, including new files. Run configured checks, require valid review approval, and verify the reviewed state has not changed before committing or pushing. Do not force-push or publish to a default branch. Require explicit push opt-in and an explicit destination.

The implementation prompt asks the model not to commit or push. If implementation has unrestricted shell access, that instruction cannot enforce a publication boundary. Before claiming that publication is gated, choose a real isolation strategy (for example, an implementation environment without Git credentials or network access, with publication outside it). A disposable worktree alone does not isolate credentials, filesystem access, or networking. Never advertise a prompt-only restriction as enforcement.

## Cost and failure semantics

No automatic application retries. Budget accounting starts fresh for each stage, but no hard cap is promised until the adapter can actually enforce one. Pi's own retries and compaction also need an explicit policy, and their usage must not disappear from totals. Preserve available output and usage on failure and stop the workflow when preparation, execution, checks, or review fails.

## Alternatives

- One Pi session implementing, reviewing, and pushing: simpler wiring, but implicit state and no independent review or application-controlled publication gate.
- Pi's TypeScript SDK: useful for Node applications; RPC avoids introducing a Node bridge solely to host a Go adapter.
- LLM-controlled Git publication: unnecessary nondeterminism for operations that Go can perform explicitly and test locally.

## Open questions

- Which provider/model should each stage use?
- Which repository and test commands will be the first target?
- What isolation will enforce the implementation/publication boundary?
- Which Git remote and feature-branch naming policy should the CLI use?
- How should libstall represent unavailable usage, cached tokens, monetary cost, and enforceable budgets?

## References

- [Pi RPC documentation](https://github.com/earendil-works/pi-mono/blob/main/packages/coding-agent/docs/rpc.md)
- [Pi CLI documentation](https://github.com/earendil-works/pi-mono/blob/main/packages/coding-agent/README.md)
- [libstall execution model](../../../libstall/docs/decisions/001-execution-model.md)
