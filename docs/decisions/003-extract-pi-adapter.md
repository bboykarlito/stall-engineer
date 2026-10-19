# ADR-003: Consume pi-in-stall as a separate Go module

## Status

Accepted. Supersedes the deferred-extraction portion of [ADR-002](002-pi-profiles.md).

## Context

The Pi adapter's RPC handling, configuration, YAML loading, and tests are independent of the engineering workflow. The user requested extraction into a sibling pi-in-stall repository. Live compatibility has not yet been verified; extraction is structural, not a claim of runtime validation or production readiness.

## Decision

Move the adapter's six Go source/test files unchanged into the root of module `github.com/bboykarlito/pi-in-stall`, package `pi`. The new repository is a sibling checkout with its own Git history, module metadata, documentation, and test commands.

stall-engineer imports that public package. Remove the tracked internal implementation and its adapter tests rather than maintaining duplicate code. Leave existing untracked user-authored package READMEs untouched; any README remaining under the old internal path is not a compiled adapter package.

Retain application behaviours, YAML profile files, CLI override precedence, read-only review checks, and Git/workflow policy here. libstall remains independent of both the application and Pi/YAML implementation.

Use explicit local replacements for both libstall and pi-in-stall. Go does not inherit a dependency's replace directives, so replacing only pi-in-stall is insufficient while libstall is unreleased. The placeholder version is v0.0.0; no remote repository, published release, or installation guarantee is implied.

## Consequences

CLI commands and YAML schema remain unchanged. A third sibling checkout is now required for local builds. Run adapter tests in pi-in-stall and application tests in stall-engineer separately. Both use fake processes/harnesses; extraction makes no paid model calls.

Future releases can replace local paths with real versioned dependencies. Permission isolation, usage availability, budget policy, live Pi smoke testing, and Git publication remain separate work.
