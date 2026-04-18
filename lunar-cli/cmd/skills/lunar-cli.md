---
name: lunar-cli
description: Lunar CLI reference for managing serverless Lua functions. Use when creating, updating, deploying, invoking, or managing Lunar functions via the CLI. Covers all lunar commands including functions, versions, executions, tokens, env vars, and KV store.
argument-hint: [command]
---

# Lunar CLI Reference

`lunar` is the CLI for the Lunar FaaS platform — a serverless runtime for Lua functions.

## Authentication

```bash
lunar login              # Device auth flow (opens browser)
lunar logout             # Remove stored token
```

Auth can also be provided via flags or env:
```bash
lunar --server https://my-server.com --token mytoken <command>
# or
LUNAR_SERVER=https://my-server.com LUNAR_TOKEN=mytoken lunar <command>
```

## Global Flags

| Flag | Env | Description |
|------|-----|-------------|
| `--server` | `LUNAR_SERVER` | Lunar server URL |
| `--token` | `LUNAR_TOKEN` | API token |
| `-o, --output` | — | `pretty` (default) or `json` |
| `--show-code` | — | Include code field in output |

## Functions

### Create a function

```bash
lunar functions create \
  --name "my-function" \
  --code 'function handler(ctx, event) return { statusCode = 200, body = "hello" } end' \
  --description "Optional description"
```

Read code from a file via stdin:
```bash
lunar functions create --name "my-function" --code - < myfunction.lua
```

### List functions

```bash
lunar functions list
```

### Get a function

```bash
lunar functions get <id>
lunar functions get <id> --show-code   # include Lua source
```

### Update a function

```bash
# Update name/description
lunar functions update <id> --name "new-name" --description "new desc"

# Deploy new code (creates a new version automatically)
lunar functions update <id> --code - < updated.lua

# Schedule with cron
lunar functions update <id> \
  --cron-schedule "*/5 * * * *" \
  --cron-status active

# Clear schedule
lunar functions update <id> --cron-schedule ""

# Disable / re-enable
lunar functions update <id> --disabled
lunar functions update <id> --disabled=false

# Configure log retention
lunar functions update <id> --retention-days 30

# Save HTTP responses for debugging
lunar functions update <id> --save-response
```

### Delete a function

```bash
lunar functions delete <id>
```

### Set environment variables

```bash
lunar functions env <id> --env KEY=VALUE --env OTHER_KEY=OTHER_VALUE
```

### Set KV store values

```bash
# Function-scoped KV
lunar functions kv <id> --kv mykey=myvalue --kv another=value

# Global KV (shared across all functions)
lunar functions kv <id> --kv sharedkey=value --global
```

### Check next scheduled run

```bash
lunar functions next-run <id>
```

## Versions

Every `functions update --code` call creates a new version. Only one version is active at a time.

```bash
lunar versions list <function-id>          # List all versions
lunar versions get <function-id> <version-id>
lunar versions get <function-id> <version-id> --show-code
lunar versions activate <function-id> <version-id>   # Roll back/forward
lunar versions diff <function-id> <version-id-a> <version-id-b>
lunar versions delete <function-id> <version-id>
```

## Invoke

Execute a function and see its response:

```bash
lunar invoke <function-id>
lunar invoke <function-id> --method POST --body '{"key":"value"}'
lunar invoke <function-id> --method POST --body - < payload.json
```

## Executions

```bash
lunar executions list <function-id>        # Recent executions
lunar executions get <function-id> <execution-id>
lunar executions logs <function-id> <execution-id>
lunar executions ai-requests <function-id> <execution-id>
lunar executions email-requests <function-id> <execution-id>
```

## API Tokens

```bash
lunar tokens list
lunar tokens revoke <token-id>
```

## Lua API Reference

```bash
lunar llms                # Print the full Lua API reference from the server
lunar skills lua          # Print the Lua function authoring guide (embedded)
```

## Typical Workflows

### Deploy a new function from a file

```bash
lunar functions create \
  --name "my-api" \
  --code - < my-api.lua
```

### Update code and verify

```bash
# Deploy new code
lunar functions update <id> --code - < my-api.lua

# Invoke to test
lunar invoke <id> --method POST --body '{"test": true}'
```

### Roll back to a previous version

```bash
lunar versions list <function-id>
lunar versions activate <function-id> <old-version-id>
```

### Configure an AI function

```bash
lunar functions env <id> --env ANTHROPIC_API_KEY=sk-ant-...
# or for OpenAI:
lunar functions env <id> --env OPENAI_API_KEY=sk-...
```
