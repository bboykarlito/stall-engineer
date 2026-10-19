# ADR-002: Pi-owned YAML profiles and eventual pi-in-stall extraction

## Status

Accepted. The decision to defer extraction is superseded by [ADR-003](003-extract-pi-adapter.md). The Pi-owned schema and application-owned profile/review policies remain in effect. The original decision below records the pre-extraction design.

## Context

libstall should remain a thin runtime-independent execution library. Consumers should be able to import maintained harness adapters or implement their own. Pi is the first adapter; Claude Code and Codex are future candidates. Their model, tool, skill, and permission semantics should not be flattened into a premature universal configuration format.

The engineering application also needs distinct implementation and review profiles without tying configuration to a particular repository directory.

## Decision

The Pi adapter's eventual module/repository name is **pi-in-stall**. Develop and exercise it in `internal/harness/pi` first, including live compatibility testing with explicit approval before extraction. These adapters will be ordinary Go modules implementing `stall.Harness`, not runtime-loaded Go plugins. libstall will not depend on them or on YAML.

The Pi adapter owns a versioned YAML `Profile`, strict loading, validation, and conversion into invocation `Config`. The schema requires `harness: pi`, `version: 1`, a model, and explicit built-in tool allowlist. Optional provider and selected skill paths stay Pi-specific. Unknown/duplicate keys, additional documents, unsupported settings, and empty/duplicate tool lists fail. The parser dependency is `go.yaml.in/yaml/v3`; do not hand-roll YAML parsing.

Profiles describe reusable settings. Application code supplies the working directory, task, context, and runner policy. Credentials are absent from the schema: use Pi authentication. No arbitrary argument passthrough, shell/environment expansion, or implicit discovery is supported.

Selected skills resolve relative to the profile file's directory. A directory means exactly its SKILL.md, not recursive discovery. Explicit Markdown files must have a name and description; duplicate selected skill names fail. Pi receives repeated `--skill` arguments while `--no-skills` remains enabled. `skills.discover: true` is rejected for this version. Skills are trusted instructions, not tool permissions or a sandbox. Validation does not pin file contents or guarantee invocation by the model.

The application ships `harnesses/implement.yaml` and `harnesses/review.yaml`. CLI flags select each profile, defaulting to these paths relative to the current working directory. Explicit `-model` and `-provider` overrides apply to both profiles; otherwise each profile retains its own values. An explicit empty provider clears the profile value, while an empty model fails validation. Missing profile files fail rather than falling back to built-in defaults.

Read-only review is an application requirement, not a generic adapter restriction. The CLI validates both profiles and rejects review tools outside read/grep/find/ls before launching implementation. This prevents a YAML edit from silently weakening the workflow's tool policy. Direct adapter consumers remain responsible for their own role requirements.

## Alternatives considered

- Universal harness YAML: rejected because capabilities and enforcement vary across runtimes.
- YAML in libstall: rejected because parsing/configuration is not core execution supervision.
- Immediate separate repository: deferred until real use validates the adapter's API and Pi compatibility.
- Ambient skill discovery: rejected to avoid depending on whichever skills happen to be installed on a machine.
- Prompt-only read-only review: rejected because tool restrictions must be applied by the runtime and guarded by application policy.

## Consequences

Profiles can be reused across repository directories, and each stage can select its own model/provider/tools/skills. Applications can consume the adapter programmatically without YAML. Installed binaries must receive explicit profile paths unless run from a directory containing the shipped profiles.

The adapter remains unsandboxed. Profiles, selected skills, runtime settings, and authentication are trusted local inputs. This increment does not add spend enforcement, credential isolation, publication, or guaranteed termination of shell descendants. No live model calls are part of profile tests.
