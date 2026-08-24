# Workspace and authored-format contract

The filesystem is the source of truth for authored Kick Sim data. Runtime history is separate and must never rewrite authored definitions.

## Layout

```text
.kick-sim/
  config.yaml
  data/users.yaml
  scenarios/**/*.yaml|yml|json
  suites/**/*.yaml|yml|json
  keys/public-key.pem
  keys/private-key.pem
  .runtime/kick-sim.db
```

The private key and `.runtime/` are ignored by the initialized workspace. Scenario and suite IDs are lowercase, extension-free relative paths. Absolute paths, traversal, empty segments, symlinks, and the reserved `builtin:` prefix are rejected for custom definitions.

## Versioned formats

`config.yaml`, `data/users.yaml`, scenarios, and suites currently use format version `1`. Parsers are strict: unknown fields, malformed values, and unsupported versions fail validation. Kick Sim does not silently discard fields or rewrite externally authored files.

Actor-owned fields are `user_id`, `username`, `channel_slug`, `is_verified`, and optional `profile_picture`. A bound actor is the only source for those fields. Event-specific context such as chat identity, badges, anonymity, replies, moderation metadata, and emotes remains scenario-owned.

Scenario files contain exactly one single-event `request` or a multi-step `steps` timeline. Suite cases reference scenario IDs and assert HTTP statuses, delivery counts, duplicate message reuse, and suite thresholds. Duplicate delivery checks describe response behavior only; they do not claim application-level idempotency.

## Migration policy

- Compatible releases may add optional fields while retaining all existing meanings.
- A required-field change, removal, or semantic reinterpretation requires a new format version.
- A binary that sees a newer authored version stops with an actionable error and leaves the file unchanged.
- Studio source saves validate first, compare the exact source revision, and atomically replace only the intended file. Invalid drafts remain memory-only.
- Runtime database migrations run transactionally after integrity validation and a recoverable backup. A database newer than the running binary is rejected without migration or replacement.
- Existing version-1 authored files remain supported throughout the stable major version. A future migration command must be explicit, preserve a backup, and never run merely because Studio opened.
