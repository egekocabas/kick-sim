# Architecture

Kick Sim is a local-first Go application with two adapters: the command-line interface and the embedded React Studio. Both use the same application service and filesystem workspace, so behavior does not drift between interactive and automated use.

## Dependency direction

```text
cmd/kick-sim → cli ──────┐
                         ├→ app → actors / config / delivery / events / history / signing / workspace
embedded web → studio ───┤
                         ├→ scenario → workflow → app
                         └→ suite ───────┘

config / delivery → loopback policy
```

The CLI and Studio translate input and render output. `internal/app` owns event-generation, delivery, history, and replay orchestration. Lower-level packages own one policy or persistence concern and do not depend on either adapter. Scenario workflows and suites call the application service rather than duplicating signing or delivery logic.

The Studio frontend treats the generated OpenAPI client as its transport boundary. Feature modules own their local React state and TanStack Query mutations; shared modules provide response parsing, query-key factories, and loading/error UI. Generated API files are not hand-edited.

## Filesystem sources of truth

A workspace configuration selects actors, destinations, signing keys, history settings, and authored directories. Custom scenarios and suites remain ordinary YAML or JSON files. Studio source editing preserves exact file text, requires the revision originally loaded, rejects symlink traversal, and replaces files atomically. Built-ins are embedded read-only assets and custom definitions can intentionally shadow neither a built-in nor another supported extension.

SQLite stores runtime history only. Opening an existing database may validate or migrate it but never silently creates an absent database. Migrations back up an existing database before modification. Retention deletes complete scenario runs so their generated events and delivery attempts are removed together.

## Safety-critical flows

### Delivery

Configuration rejects non-HTTP schemes, credentials, fragments, and non-loopback hostnames. Delivery repeats the policy at runtime: proxies are disabled, every resolved address must be loopback, redirects are denied, and per-delivery idle connections are closed. The runtime resolution check is required even after configuration validation because DNS answers can change.

### Studio control

Studio binds only to an explicit loopback IP and checks the exact request host. Mutations require JSON, an accepted origin when present, and a random per-process control token. Browser requests receive that token through a strict, HTTP-only cookie; non-browser requests must present it as a bearer token. The browser never receives the private signing key.

### Signing-key rotation

Key reads and rotations are serialized within the process. Rotation writes and syncs staged files, retains both old-key backups while installing and validating the new pair, and restores both files after any ordinary installation or validation failure. The key directory is synced after installation or rollback so the pair is not reported as durable prematurely.

### Deterministic generation

Payload composition applies contract defaults, workspace actors, scenario actors, scenario data, explicit overrides, dynamic values, then schema validation. Map-backed override pointers are sorted before application. Workflows advance logical time even when wall-clock waits are skipped, which keeps generated timestamps and tests reproducible.

## Development checks

Run `make test` for generated-contract drift, Go race tests and vet, frontend linting, strict typechecking, unit tests, production bundling, and embedded-asset tests. The CI workflow additionally runs Staticcheck, dead-code analysis, vulnerability and secret scans, Playwright Studio coverage, and a GoReleaser snapshot build.
