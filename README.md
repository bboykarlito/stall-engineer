# stall-engineer

A minimal engineering agent built with [libstall](https://github.com/bboykarlito/libstall) and Pi.

Intended workflow: accept a task → implement → independently review → commit and push an explicitly selected feature branch. Ordinary Go code will own orchestration.

## Status

First increment only: implementation and review behaviours with validation tests. There is no executable, Pi adapter, runner execution, or Git publication yet. Nothing currently invokes a model, modifies a target repository, or pushes changes.

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

## Next increments

1. Add tested runner lifecycle execution to libstall, with explicit policy limitations rather than a fictitious hard spend guarantee.
2. Implement and test a Pi subprocess adapter against recorded/fake protocol events before making paid model calls. Resolve usage availability and cost representation in libstall first.
3. Wire the stages in a CLI with explicit target repository, provider/model, and timeout settings.
4. Add guarded Git publication and tests using temporary local repositories; require explicit opt-in to push.

See [ADR-001](docs/decisions/001-engineering-workflow.md) for the proposed execution and publication boundaries.
