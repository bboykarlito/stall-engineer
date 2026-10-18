# stall-engineer

A minimal engineering agent built with [libstall](https://github.com/bboykarlito/libstall) and Pi.

Intended workflow: accept a task → implement → independently review → commit and push an explicitly selected feature branch. Ordinary Go code will own orchestration.

## Status

Implemented: implementation and review behaviours, Pi invocation configuration, and validation tests. There is no executable, Pi process adapter, runner execution, or Git publication yet. Nothing currently invokes a model, modifies a target repository, or pushes changes.

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

`internal/harness/pi` supplies configuration for the future subprocess adapter:

```go
implementation := pi.ImplementationConfig(absoluteRepoPath)
review := pi.ReviewConfig(absoluteRepoPath)

// Optional overrides; otherwise Pi resolves the model using its configured providers.
implementation.Provider = "your-provider-id"
implementation.Model = "gpt-5.6-terra"

args, err := implementation.Args()
// Handle err. The future adapter will run pi with args and set Cmd.Dir to WorkDir.
```

Both configurations default to `gpt-5.6-terra`. Model availability and credentials have not been verified with a live call. Provider is intentionally not guessed.

The generated arguments select RPC mode, disable session persistence and project-resource trust, disable discovery of extensions/skills/templates/context files, and explicitly allowlist tools. Implementation permits `read,grep,find,ls,edit,write,bash`; review permits only `read,grep,find,ls`. Each constructor returns an independent tool slice. Configuration remains mutable, so callers changing the review allowlist are responsible for preserving its read-only policy.

`Args` requires an absolute existing working directory, a nonempty model, and a nonempty list of supported built-in tools. It does not verify that the directory is a Git repository. WorkDir is process configuration, not a CLI argument or filesystem sandbox. Shell access in the implementation profile can bypass prompt-only publication restrictions.

These flags do not reset all global Pi settings, enforce budgets, or configure automatic retries/compaction. Those policies remain work for the process adapter. Repository instructions must be passed deliberately because automatic context-file discovery is disabled.

## Next increments

1. Add tested runner lifecycle execution to libstall, with explicit policy limitations rather than a fictitious hard spend guarantee.
2. Implement and test a Pi subprocess adapter against recorded/fake protocol events before making paid model calls. Resolve usage availability and cost representation in libstall first.
3. Wire the stages in a CLI with explicit target repository, provider/model, and timeout settings.
4. Add guarded Git publication and tests using temporary local repositories; require explicit opt-in to push.

See [ADR-001](docs/decisions/001-engineering-workflow.md) for the proposed execution and publication boundaries.
