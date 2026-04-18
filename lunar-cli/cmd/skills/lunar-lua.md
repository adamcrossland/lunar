---
name: lunar-lua
description: Lua function authoring guide for the Lunar FaaS platform. Use when writing, reviewing, or debugging Lunar Lua functions. Covers the handler signature, all stdlib modules (log, kv, env, http, json, base64, crypto, time, url, strings, ai, email, random, router), and common patterns.
argument-hint: [module or pattern]
---

# Lunar Lua Function Guide

Every function is a `.lua` file that defines a `handler` function. The runtime calls it on each HTTP request.

> For the full, always-up-to-date API reference run `lunar llms` or fetch `/llms.txt` from your Lunar server.

## Handler Signature

```lua
function handler(ctx, event)
  return {
    statusCode = 200,
    body = "response text",
    headers = { ["Content-Type"] = "text/plain" },
    isBase64Encoded = false  -- optional
  }
end
```

### ctx — execution metadata

| Field | Type | Description |
|-------|------|-------------|
| `ctx.executionId` | string | Unique ID for this execution |
| `ctx.functionId` | string | Function identifier |
| `ctx.functionName` | string | Function name |
| `ctx.version` | string | Function version |
| `ctx.requestId` | string | HTTP request ID |
| `ctx.startedAt` | number | Unix timestamp (seconds) |
| `ctx.baseUrl` | string | Base URL of the deployment |

### event — incoming HTTP request

| Field | Type | Description |
|-------|------|-------------|
| `event.method` | string | HTTP method (`GET`, `POST`, etc.) |
| `event.path` | string | Full path including `/fn/{id}` prefix |
| `event.relativePath` | string | Path without `/fn/{id}` prefix |
| `event.body` | string | Request body as string |
| `event.headers` | table | Request headers |
| `event.query` | table | Query parameters |

## Standard Library

### log — structured logging

```lua
log.info("message")
log.debug("message")
log.warn("message")
log.error("message")
```

### kv — persistent key-value storage

Storage is scoped to the function by default. Global store is shared across all functions.

```lua
-- Function-scoped
local val = kv.get("key")            -- string | nil
kv.set("key", "value")               -- boolean
kv.delete("key")                     -- boolean
kv.listKeys()                        -- string[]
kv.all()                             -- table | nil

-- Global store
kv.getGlobal("key")                  -- string | nil
kv.setGlobal("key", "value")         -- boolean
kv.deleteGlobal("key")               -- boolean
kv.listGlobalKeys()                  -- string[]
kv.allGlobal()                       -- table | nil
```

### env — environment variables

Scoped to the function. Set via `lunar functions env <id> --env KEY=VALUE`.

```lua
local val = env.get("MY_KEY")        -- string | nil
env.set("MY_KEY", "value")           -- boolean
env.delete("MY_KEY")                 -- boolean
```

### http — outbound HTTP

```lua
local res, err = http.get(url, options)
local res, err = http.post(url, options)
local res, err = http.put(url, options)
local res, err = http.delete(url, options)
```

Options:
```lua
{
  headers = { ["Authorization"] = "Bearer token" },
  query   = { ["param"] = "value" },
  body    = "request body"   -- POST/PUT only
}
```

Response:
```lua
res.statusCode  -- number
res.body        -- string
res.headers     -- table
```

### json — encode / decode

```lua
local str, err = json.encode(value)
local val, err = json.decode(str)
```

### base64 — encode / decode

```lua
local encoded = base64.encode("hello")
local decoded, err = base64.decode(encoded)
```

### crypto — hashing, HMAC, UUID

```lua
-- Hashes (hex string)
crypto.md5(str)
crypto.sha1(str)
crypto.sha256(str)
crypto.sha512(str)

-- HMAC (hex string)
crypto.hmac_sha1(message, key)
crypto.hmac_sha256(message, key)
crypto.hmac_sha512(message, key)

-- UUID v4
local id = crypto.uuid()
```

### time — timestamps and formatting

Uses Go's reference time layout: `2006-01-02 15:04:05`

```lua
time.now()                              -- Unix seconds (number)
time.format(timestamp, layout)          -- string
time.parse(str, layout)                 -- number | nil, error | nil
time.sleep(milliseconds)
```

Common layouts:
- `"2006-01-02"` — date only
- `"2006-01-02 15:04:05"` — date + time
- `"2006-01-02T15:04:05Z"` — ISO 8601

### url — parsing and encoding

```lua
local parsed, err = url.parse("https://example.com/api?k=v")
-- parsed.scheme, .host, .path, .fragment, .query, .username, .password

local encoded = url.encode("hello world")    -- "hello+world"
local decoded, err = url.decode("hello+world")
```

### strings — utilities

```lua
strings.trim(s)
strings.trimLeft(s)
strings.trimRight(s)
strings.split(s, sep)             -- table
strings.join(arr, sep)            -- string
strings.hasPrefix(s, prefix)      -- boolean
strings.hasSuffix(s, suffix)      -- boolean
strings.contains(s, substr)       -- boolean
strings.replace(s, old, new, n)   -- n=-1 replaces all
strings.toLower(s)
strings.toUpper(s)
strings.repeat(s, count)
```

### ai — LLM chat completions

Requires `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` set via `lunar functions env`.

```lua
local res, err = ai.chat({
  provider    = "anthropic",           -- "openai" or "anthropic"
  model       = "claude-haiku-4-5-20251001",
  messages    = {
    { role = "system", content = "You are helpful" },
    { role = "user",   content = userMessage }
  },
  max_tokens  = 1024,                  -- optional, default 1024
  temperature = 0.7,                   -- optional
  endpoint    = "https://custom.api"   -- optional override
})

-- res.content, res.model, res.usage.input_tokens, res.usage.output_tokens
```

Recommended models:
- Anthropic: `claude-haiku-4-5-20251001` (fast/cheap), `claude-sonnet-4-6` (balanced), `claude-opus-4-6` (best)
- OpenAI: `gpt-4o-mini`, `gpt-4o`

### email — send via Resend

Requires `RESEND_API_KEY` set via `lunar functions env`.

```lua
local res, err = email.send({
  from       = "sender@yourdomain.com",  -- required
  to         = "user@example.com",       -- string or table
  subject    = "Hello!",                 -- required
  html       = "<p>Body</p>",            -- required if no text
  text       = "Plain text",             -- required if no html
  cc         = "cc@example.com",         -- optional
  bcc        = {"bcc@example.com"},      -- optional
  reply_to   = "reply@example.com",      -- optional
  headers    = { ["X-Custom"] = "v" },   -- optional
  tags       = {{name="n", value="v"}},  -- optional
  scheduled_at = time.now() + 3600       -- optional (Unix ts or ISO 8601)
})
-- res.id  (Resend email ID)
```

### random — secure random generation

```lua
random.int(min, max)      -- integer in [min, max] inclusive
random.float()            -- float in [0.0, 1.0)
random.string(length)     -- alphanumeric string
random.bytes(length)      -- base64-encoded random bytes; returns str, err
random.hex(length)        -- hex-encoded random bytes; returns str, err
random.id()               -- globally unique sortable 20-char xid
```

### router — path matching and URL building

```lua
-- Match and extract
router.match("/users/42", "/users/:id")          -- true
router.params("/users/42", "/users/:id")         -- {id = "42"}

-- Wildcard (must be at end)
router.match("/files/a/b/c", "/files/*")         -- true

-- Build paths (adds /fn/{functionId} prefix)
router.path("/users/:id", {id = "42"})           -- "/fn/{id}/users/42"
router.url("/users/:id",  {id = "42"})           -- "http://host/fn/{id}/users/42"
```

## Common Patterns

### Parse JSON body

```lua
local data, err = json.decode(event.body)
if err then
  return { statusCode = 400, body = json.encode({ error = "Invalid JSON: " .. err }) }
end
```

### JSON response helper

```lua
local function jsonResponse(status, data)
  return {
    statusCode = status,
    headers    = { ["Content-Type"] = "application/json" },
    body       = json.encode(data)
  }
end
```

### Simple REST router

```lua
function handler(ctx, event)
  local path   = event.relativePath
  local method = event.method

  if method == "GET" and router.match(path, "/users/:id") then
    local params = router.params(path, "/users/:id")
    return jsonResponse(200, { id = params.id })
  end

  if method == "POST" and path == "/users" then
    local data, err = json.decode(event.body)
    if err then return jsonResponse(400, { error = "bad json" }) end
    return jsonResponse(201, { created = true })
  end

  return jsonResponse(404, { error = "not found" })
end
```

### Counter with KV

```lua
function handler(ctx, event)
  local count = tonumber(kv.get("counter") or "0") + 1
  kv.set("counter", tostring(count))
  return { statusCode = 200, body = json.encode({ count = count }) }
end
```

### Authenticated outbound call

```lua
function handler(ctx, event)
  local apiKey = env.get("API_KEY")
  if not apiKey then
    return { statusCode = 500, body = json.encode({ error = "API_KEY not set" }) }
  end

  local res, err = http.get("https://api.example.com/data", {
    headers = { ["Authorization"] = "Bearer " .. apiKey }
  })
  if err then
    log.error("request failed: " .. err)
    return { statusCode = 502, body = json.encode({ error = err }) }
  end

  return { statusCode = res.statusCode, body = res.body,
           headers = { ["Content-Type"] = "application/json" } }
end
```

### AI chat endpoint

```lua
function handler(ctx, event)
  local data, err = json.decode(event.body)
  if err then return { statusCode = 400, body = json.encode({ error = "bad json" }) } end

  local res, err = ai.chat({
    provider = "anthropic",
    model    = "claude-haiku-4-5-20251001",
    messages = {
      { role = "system", content = "You are a helpful assistant." },
      { role = "user",   content = data.message }
    }
  })
  if err then
    log.error("ai.chat failed: " .. err)
    return { statusCode = 500, body = json.encode({ error = err }) }
  end

  return {
    statusCode = 200,
    headers    = { ["Content-Type"] = "application/json" },
    body       = json.encode({ reply = res.content, tokens = res.usage.input_tokens + res.usage.output_tokens })
  }
end
```

## Error Handling Rules

- Functions that return `(result, error)` return `nil, "error string"` on failure — always check `err`
- KV and env writes return a boolean — check if you need to know they succeeded
- `json.decode` on an empty or nil body will error — guard with `if event.body and #event.body > 0`

## Execution Environment

- Default timeout: 5 minutes (configurable per function)
- KV storage is scoped to `functionId` by default; global store is shared
- `env` storage is scoped to `functionId`
- `crypto/rand` is used for all random generation (cryptographically secure)
