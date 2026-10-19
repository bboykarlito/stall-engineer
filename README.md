# stall-engineer

A minimal engineering agent built with [libstall](https://github.com/bboykarlito/libstall) and Pi.

Intended workflow: accept a task → implement → independently review → commit and push an explicitly selected feature branch. Ordinary Go code will own orchestration.

## Status

Implemented: implementation and review behaviours, Pi RPC subprocess execution, invocation configuration, and tests using fake subprocesses. There is no CLI executable, runner execution, or Git publication yet. Calling the adapter can invoke a real model and tools; development tests do neither. Live Pi/model compatibility has not been verified.

The review gate requires a structured, explicit approval with a nonempty summary and no blocking findings. This validates the report, not the truth of its claims. Implementation validation currently checks only that a summary exists.

## Development

Requires Go 1.24.4 or later and sibling checkouts:

```text
Projects/
  libstall/
  stall-engineer/
```

The local `go.mod` replacement uses `../libstall` while the library is under development. The application module uses the temporary local name `stall-engineer`; no hosted repository or remote has been created.

```sh
go test ./...
go vet ./...
```

## Pi harness configuration

`internal/harness/pi` supplies configuration and a `stall.Harness` implementation:

```go
implementation := pi.ImplementationConfig(absoluteRepoPath)
review := pi.ReviewConfig(absoluteRepoPath)

// Optional overrides; otherwise Pi resolves the model using its configured providers.
implementation.Provider = "your-provider-id"
implementation.Model = "gpt-5.6-terra"

harness, err := pi.New(implementation)
// Handle err. New validates and snapshots configuration, without starting Pi.
// result, err := harness.Execute(ctx, preparedRequest)
```

Both configurations default to `gpt-5.6-terra`. Model availability and credentials have not been verified with a live call. Provider is intentionally not guessed.

The generated arguments select RPC mode, disable session persistence and project-resource trust, disable discovery of extensions/skills/templates/context files, and explicitly allowlist tools. Implementation permits `read,grep,find,ls,edit,write,bash`; review permits only `read,grep,find,ls`. Each constructor returns an independent tool slice. Configuration remains mutable, so callers changing the review allowlist are responsible for preserving its read-only policy.

`Args` requires an absolute existing working directory, a nonempty model, and a nonempty list of supported built-in tools. It does not verify that the directory is a Git repository. WorkDir is process configuration, not a CLI argument or filesystem sandbox. Shell access in the implementation profile can bypass prompt-only publication restrictions.

These flags do not reset all global Pi settings or enforce budgets. Repository instructions must be passed deliberately because automatic context-file discovery is disabled.

## Pi execution

`pi.New(config)` snapshots configuration. Each `Execute` starts a fresh `pi --mode rpc` process in `WorkDir`, disables automatic retry and compaction through acknowledged RPC commands, and sends the instructions and task as an explicit user message. It waits for `agent_settled` (not merely `agent_end` or prompt acceptance), checks assistant completion, requests session token statistics, and terminates/reaps the RPC process.

Use a context with a deadline. Cancellation closes pipes and kills/reaps Pi, but does not guarantee termination of every shell-tool descendant. There is no OS sandbox or hard spend cap. Model authentication uses the existing Pi setup and environment.

The adapter returns the last completed assistant message's text. Provider errors, aborted/truncated responses, missing completion, malformed protocol data, missing statistics, and unexpected UI requests fail explicitly. Output already received remains available on errors. Diagnostic errors include up to 8 KiB of stderr; treat logs accordingly. JSONL records are limited to 16 MiB.

Token accounting uses session totals: input includes uncached input plus cache-read and cache-write tokens; output is reported separately. Monetary cost is not represented. If execution fails before statistics are retrieved, usage remains zero and must be treated as **unknown, not free**. The current libstall type cannot express availability; this prototype is not suitable for budget enforcement. A successful statistics response also cannot prove that a provider supplied accurate billing data.

Tests exercise RPC acceptance versus completion, Unicode line separators, usage, provider failures, malformed output, premature exit, UI rejection, configuration snapshots, and cancellation through fake child processes. No paid API calls are made.

## Next increments

1. Add tested runner lifecycle execution to libstall, with explicit policy limitations rather than a fictitious hard spend guarantee.
2. Resolve usage availability and cost representation in libstall before enabling budget policy; verify the adapter against a live Pi model only with explicit approval.
3. Wire the stages in a CLI with explicit target repository, provider/model, and timeout settings.
4. Add guarded Git publication and tests using temporary local repositories; require explicit opt-in to push.

See [ADR-001](docs/decisions/001-engineering-workflow.md) for the proposed execution and publication boundaries.
