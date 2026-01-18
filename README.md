# Veriflow

Veriflow is a simple yet powerful CLI tool for defining and running end-to-end API test flows using a simple JSON configuration file.

## Index

- [Overview](#overview)
    - [Why Veriflow](#why-veriflow)
    - [Project Status](#project-status)
    - [Concepts](#concepts)
    - [Installation](#installation)
    - [Quick Start](#quick-start)
    - [Design Principles](#design-principles)
- [Documentation](#documentation)
    - [Configuration Reference](#configuration-reference)
    - [CLI Reference](#cli-reference)
    - [Bindings](#bindings)
    - [Hooks](#hooks)
    - [Session Handling](#session-handling)
    - [Exit Codes](#exit-codes)
- [Contributing](#contributing)
- [License](#license)

## Overview

### Why Veriflow

Because it's the simplest form of targeted flow testing that:

- Treats your API as a black box
- Runs ordered, stateful flows (auth -> action -> verify)
- Passes data between steps (JWTs, IDs, etc.)
- Requires zero code changes in your server

If your API works, Veriflow passes. If it doesn't, it fails. Simple.

### Project Status

Veriflow is production-ready with continuous features and general enhancements.

Expect:

- Minor config changes
- Better error messages
- Improved reporting

Core concepts are stable.

### Concepts

#### Flow

Represents a real user or system journey.

Example:

- user-onboarding
- checkout
- admin-create-user

Flows are ordered. If a step fails, the flow stops.

#### Step

A step is a single HTTP request with assertions.

Each step defines:

- HTTP method + path
- Request body / headers / files
- Assertions on the response
- Optional exported values for the next steps

#### Exports

You can extract values from a response and reuse them later.

Common use cases:

- JWT tokens
- User ID created in a test to be asserted in the following GET request

Exports are resolved via JSONPath.

#### Assertions

Veriflow supports:

- Status code checks
- JSONPath existence
- JSONPath equality / containment checks

Assertions are explicit. No magic.

### Installation

```bash
go install github.com/okira-e/veriflow@latest
```

Or download a prebuilt binary from releases.

### Quick Start

```bash
veriflow init
```

This creates a veriflow.json at the project root.

```bash
veriflow run
```

Runs all flows.

```bash
veriflow run user-onboarding
```

Runs a single flow.

### Design Principles

- Black-box testing only
- Deterministic execution
- Explicit over implicit
- No framework assumptions
- Human-readable configs

## Documentation

### Configuration Reference

The configuration file (`veriflow.json`) has the following structure:

```json
{
    "projectName": "my-api-tests",
    "baseUrl": "http://localhost:3000",
    "beforeRun": ["echo 'Starting tests'"],
    "afterRun": ["echo 'Tests complete'"],
    "flows": [
        {
            "name": "user-onboarding",
            "steps": [...]
        }
    ]
}
```

#### Top-Level Fields

| Field         | Type     | Required | Description                                      |
| ------------- | -------- | -------- | ------------------------------------------------ |
| `projectName` | string   | No       | Name of the project                              |
| `baseUrl`     | string   | Yes      | Base URL for all requests                        |
| `beforeRun`   | string[] | No       | Hook: shell commands to run before tests start   |
| `afterRun`    | string[] | No       | Hook: shell commands to run after tests complete |
| `flows`       | array    | Yes      | Array of flow objects                            |

#### Flow Object

| Field   | Type   | Required | Description                    |
| ------- | ------ | -------- | ------------------------------ |
| `name`  | string | Yes      | Unique identifier for the flow |
| `steps` | array  | Yes      | Ordered array of step objects  |

#### Step Object

| Field     | Type   | Required | Description                            |
| --------- | ------ | -------- | -------------------------------------- |
| `name`    | string | Yes      | Unique identifier within the flow      |
| `request` | object | Yes      | HTTP request configuration             |
| `assert`  | object | Yes      | Assertions to validate the response    |
| `exports` | object | No       | Variables to extract from the response |
| `options` | object | No       | Step-level options                     |

#### Request Object

| Field            | Type    | Required | Description                                                |
| ---------------- | ------- | -------- | ---------------------------------------------------------- |
| `method`         | string  | Yes      | HTTP method (GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD) |
| `path`           | string  | Yes      | Request path (appended to baseUrl)                         |
| `json`           | object  | No       | JSON request body                                          |
| `disableHeaders` | boolean | No       | If true, disables automatic cookie handling                |

#### Assert Object

| Field    | Type    | Required | Description                                |
| -------- | ------- | -------- | ------------------------------------------ |
| `status` | integer | Yes      | Expected HTTP status code                  |
| `all`    | array   | No       | Array of assertion objects (all must pass) |

#### Assertion Object

| Field      | Type    | Required | Description                                            |
| ---------- | ------- | -------- | ------------------------------------------------------ |
| `jsonpath` | string  | Yes      | JSONPath expression to evaluate                        |
| `exists`   | boolean | No       | Assert the path exists (true) or doesn't exist (false) |
| `equals`   | string  | No       | Assert the value equals this string                    |
| `contains` | string  | No       | Assert the value contains this substring               |

#### Exports Object

A map of variable names to JSONPath expressions:

```json
{
    "user_id": "$.data.user.id",
    "token": "$.data.token"
}
```

#### Step Options

| Field     | Type   | Description                                               |
| --------- | ------ | --------------------------------------------------------- |
| `timeout` | string | Request timeout (e.g., "5s", "100ms", "1m"). Default: 30s |

### CLI Reference

#### Global Flags

These flags work with all commands:

| Flag                | Short | Description                               |
| ------------------- | ----- | ----------------------------------------- |
| `--config`          |       | Config file path (default: veriflow.json) |
| `--json-output`     |       | Output machine-readable JSON              |
| `--no-color`        |       | Disable colored output                    |
| `--non-interactive` |       | Disable interactive prompts               |
| `--verbose`         | `-v`  | Enable verbose output                     |
| `--silent`          |       | Suppress all output except errors         |

The `NO_COLOR` environment variable is also respected.

#### veriflow init

Creates a new `veriflow.json` configuration file.

```bash
veriflow init

# Anything can be non interactive with required flags
veriflow init --base-url "https://base.com" --non-interactive
```

#### veriflow run

Run flows defined in the configuration.

| Flag                   | Description                                           |
| ---------------------- | ----------------------------------------------------- |
| `--base-url`           | Override the baseUrl from config                      |
| `--skip`               | Skip specific flows or steps (repeatable)             |
| `--keep-going`         | Continue running even if tests fail                   |
| `--show-full-response` | Display entire server response payload on error       |
| `--show-hooks`         | Print stdout/stderr from beforeRun and afterRun hooks |
| `--skip-hooks`         | Skip executing beforeRun and afterRun hooks           |

Examples:

```bash
# Run all flows
veriflow run

# Run specific flow(s)
veriflow run user-onboarding checkout

# Run a specific step
veriflow run user-onboarding/register

# Mixed targets. Specific flows with specific steps
veriflow run user-onboarding checkout/payment

# Run veriflow against a specific config
veriflow run --config veriflow-configs/seeding-flows.json

# Override base URL
veriflow run --base-url http://staging.example.com

# Skip specific tests
veriflow run --skip user-onboarding/register --skip checkout

# Continue on failures
veriflow run --keep-going

# CI mode
veriflow run --json-output --no-color --non-interactive
```

#### veriflow flow add

Add a new flow to the configuration.

```bash
# Interactive
veriflow flow add

# With name
veriflow flow add user-onboarding
```

| Flag        | Description                                        |
| ----------- | -------------------------------------------------- |
| `--no-save` | Modify config in memory only (don't write to disk) |

#### veriflow flow delete

Delete a flow from the configuration.

```bash
# Interactive
veriflow flow delete

# With name
veriflow flow delete user-onboarding
```

| Flag        | Short | Description                  |
| ----------- | ----- | ---------------------------- |
| `--yes`     | `-y`  | Skip confirmation prompt     |
| `--no-save` |       | Modify config in memory only |

#### veriflow step add

Add a new step to a flow.

```bash
# Interactive
veriflow step add

# With name
veriflow step add register

# Fully specified (for scripts/CI)
veriflow step add register \
  --flow user-onboarding \
  --method POST \
  --path /auth/register \
  --json '{"email": "test@example.com"}' \
  --status 201 \
  --assert "exists $.data.id" \
  --assert "equals $.data.email test@example.com" \
  --export "user_id $.data.id" \
  --non-interactive
```

| Flag        | Description                       |
| ----------- | --------------------------------- |
| `--flow`    | Flow this step belongs to         |
| `--method`  | HTTP method                       |
| `--path`    | Request path                      |
| `--json`    | JSON body                         |
| `--status`  | Expected HTTP status code         |
| `--assert`  | Assertion expression (repeatable) |
| `--export`  | Export expression (repeatable)    |
| `--no-save` | Modify config in memory only      |

Assertion syntax:

```
exists <jsonpath>
equals <jsonpath> <value>
contains <jsonpath> <value>
```

Export syntax:

```
<varname> <jsonpath>
```

### Bindings

Bindings are template variables that get replaced at runtime. They can be used in request bodies, paths, and assertion values.

Bindings can be referenced and used from a different flow but they have to be defined first.

#### Built-in Bindings

| Binding          | Description                                |
| ---------------- | ------------------------------------------ |
| `{{RUN_ID}}`     | Unique identifier for the current test run |
| `{{RAND_DIGIT}}` | Random single digit (0-9)                  |

Example:

```json
{
    "email": "test-{{RUN_ID}}@example.com",
    "code": "{{RAND_DIGIT}}{{RAND_DIGIT}}{{RAND_DIGIT}}{{RAND_DIGIT}}"
}
```

#### User-Defined Bindings (Exports)

Use `{{bind:variable_name}}` to reference exported values from previous steps:

```json
{
    "name": "register",
    "exports": {
        "user_id": "$.data.id"
    }
}
```

```json
{
    "name": "get-user",
    "request": {
        "path": "/users/{{bind:user_id}}"
    }
}
```

Bindings are case-sensitive. `{{bind:user_id}}` and `{{bind:User_Id}}` are different.

### Hooks

Hooks are shell commands that run before and after your test suite.

```json
{
    "beforeRun": ["docker-compose up -d", "sleep 2"],
    "afterRun": ["docker-compose down"]
}
```

- `beforeRun`: Executes before any tests run (e.g., start services, seed database)
- `afterRun`: Executes after all tests complete (e.g., cleanup)

Commands run in order. If a `beforeRun` command fails, tests do not run.

Use `--skip-hooks` to skip hook execution, or `--show-hooks` to see their output.

### Session Handling

Veriflow automatically maintains cookies across steps within a run. This enables stateful flows like:

1. Login (receives session cookie)
2. Protected action (cookie sent automatically)
3. Logout

To disable automatic cookie handling for a specific step, set `disableHeaders: true` in the request.

### Exit Codes

| Code     | Meaning                        |
| -------- | ------------------------------ |
| 0        | All tests passed               |
| 1        | One or more assertions failed  |
| Non-zero | Configuration or runtime error |

## Contributing

Feel free to suggest improvements, start a discussion, file an issue, or open a PR.

## License

This project is licensed under the MIT license - see the [LICENSE](LICENSE) file for details.
