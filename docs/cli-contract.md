# CLI contract

Kick Sim's stable command-line interface is the automation surface for local webhook tests. Commands read a `.kick-sim` workspace, never contact Kick, and restrict webhook delivery to loopback destinations.

## Global behavior

`--workspace` selects an explicit workspace. Without it, Kick Sim uses `KICK_SIM_WORKSPACE` or searches parent directories for `.kick-sim`. `--output human` is the interactive default; `--output json` is the stable machine-readable mode for commands that return data.

Standard output contains the requested human or JSON result. Diagnostics and verbose workspace information use standard error. JSON output contains one complete JSON value followed by a newline.

The stable exit codes are:

| Code | Meaning |
|---:|---|
| `0` | Success, including a suite whose configured thresholds pass |
| `1` | Unexpected internal failure |
| `2` | Invalid command, flag, argument, or output format |
| `3` | Workspace, configuration, or key failure |
| `4` | Invalid event, actor, scenario, timeline, or suite definition |
| `5` | Delivery, response assertion, workflow, or suite-threshold failure |

## Command groups

- `init`, `workspace *`: create, resolve, inspect, and validate a workspace.
- `event *`: list contracts, generate payloads, validate JSON, or deliver one event.
- `scenario *`: list, inspect, copy, validate, and run single-event or timeline scenarios.
- `suite *`: list, inspect, validate, and run suites.
- `history *`: inspect, replay, and delete retained delivery history.
- `keys *`: inspect, print, initialize, or rotate the simulator key pair.
- `config *`, `compatibility`, `version`, `openapi`: inspect stable configuration and build contracts.
- `studio`: start the embedded loopback-only Studio.

Commands and existing JSON fields are retained throughout the stable major version. Additive JSON fields and new commands may be introduced in compatible releases. Scripts must ignore unknown JSON fields. Removing or renaming commands, changing exit-code meaning, or changing an existing field's meaning requires a new major version or a documented compatibility transition.

Direct event and scenario commands are one-shot operations. They do not require Studio, OAuth, subscriptions, or persistent platform state.
