# stall-engineer

A minimal engineering agent built with [libstall](https://github.com/bboykarlito/libstall) and Pi.

Intended workflow: accept a task → implement → independently review → commit and push an explicitly selected feature branch. Ordinary Go code will own orchestration.

## Status

Implemented: a local-only CLI, runner-driven implementation and independent review, repository preflight and change capture, Pi RPC subprocess execution, and tests using fake harnesses/subprocesses and temporary Git repositories. Git publication is not implemented. Running the CLI invokes a real model and tools; development tests do not. Live Pi/model compatibility has not been verified.

The review gate requires a structured, explicit approval with a nonempty summary and no blocking findings. This validates the report, not the truth of its claims. Implementation validation currently checks only that a summary exists.

## Run the local-only workflow

Prerequisites: Go, Git, an authenticated Pi installation on PATH, and access to the selected model. Start from a clean repository root on an existing feature branch. Use a disposable, trusted checkout without publication credentials; implementation has unrestricted shell access.

```sh
go run ./cmd/stall-engineer \
  -repo /absolute/path/to/repository \
  -task "Fix the parser's handling of empty input" \
  -model gpt-5.6-terra \
  -timeout 10m
```

Options:

| Flag | Default | Meaning |
| --- | --- | --- |
| `-repo` | required | Repository worktree root; relative paths are also accepted. |
| `-task` | required | Task text. |
| `-model` | `gpt-5.6-terra` | Model used by both independent sessions. |
| `-provider` | Pi resolves | Optional explicit provider ID. |
| `-timeout` | `10m` | Fresh timeout for each model stage, not the complete workflow. |
| `-base-branch` | `main` | Additional protected branch; set this to your repository's base branch. |

Preflight rejects dirty/staged/untracked files, detached HEAD, submodule repositories, and protected branches (`main`, `master`, `trunk`, the configured base, and locally known `origin/HEAD`). It does not contact a remote to discover its default branch. Unknown custom defaults must be supplied with `-base-branch`.

The application runs implementation through `stall.Runner`, captures the tracked working-tree diff against HEAD plus non-ignored untracked files, and passes that evidence, the original task, and the implementation summary to a fresh read-only review session. Evidence includes binary patches and is capped at 4 MiB. External Git diff/text conversion commands are disabled. Ignored files are excluded; repositories with submodules are rejected rather than presenting incomplete nested change evidence.

Review must explicitly approve with no blocking findings. The application then verifies that HEAD, branch, and captured change evidence still match. This is a local snapshot check, not a repository lock or a guarantee against concurrent edits. Keep other writers away during execution. Index-only differences that leave the final working-tree contents unchanged are not independently reviewed.

Failures stop the workflow and return a nonzero exit status, preserving available results and leaving edits for manual inspection. Ctrl+C cancels execution. There are no automatic retries, fix loops, rollbacks, commits, or pushes. No deterministic test command is enforced yet: implementation is instructed to run checks, and review examines its claims. Verify tests yourself before publishing.

**Safety:** the application never commits or pushes, but implementation's shell tool could do so despite its instructions. This is not a sandbox or an enforceable publication boundary. A feature branch alone does not protect credentials, other files, or remote repositories. No spend cap is enforced.

## Development

Requires Go 1.24.4 or later and sibling checkouts:

```text
Projects/
  libstall/
  stall-engineer/
```

The local `go.mod` replacement uses `../libstall` while the library is under development. The application module uses the temporary local name `stall-engineer`; no hosted repository or remote has been created.

```sh
go test -race ./...
go vet ./...
go run ./cmd/stall-engineer -help
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

1. Resolve usage availability and cost representation in libstall before enabling budget policy; verify the adapter against a live Pi model only with explicit approval.
2. Add deterministic configured checks and an isolated implementation environment without publication credentials/network access.
3. Add guarded Git publication and tests using temporary local repositories; require explicit opt-in to push.

See [ADR-001](docs/decisions/001-engineering-workflow.md) for the proposed execution and publication boundaries.
