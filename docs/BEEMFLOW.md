# BeemFlow: Comprehensive Protocol & Implementation Guide

> **GitHub Actions for every business process — text-first, AI-native, open-source.**

BeemFlow is a workflow protocol, runtime, and global tool registry for the age of LLM co-workers. This document is the canonical, exhaustive, and LLM-ingestible reference for BeemFlow, suitable for training, implementation, and integration by AI agents and developers worldwide.

---

## Table of Contents
- [BeemFlow: Comprehensive Protocol \& Implementation Guide](#beemflow-comprehensive-protocol--implementation-guide)
  - [Table of Contents](#table-of-contents)
  - [1. What is BeemFlow?](#1-what-is-beemflow)
  - [2. Vision \& Philosophy](#2-vision--philosophy)
  - [3. Protocol Overview](#3-protocol-overview)
  - [4. YAML/JSON/Struct Grammar](#4-yamljsonstruct-grammar)
    - [YAML Example](#yaml-example)
    - [JSON Example](#json-example)
    - [Go Struct Example](#go-struct-example)
  - [5. Flow Anatomy \& Execution Model](#5-flow-anatomy--execution-model)
    - [Step Definition](#step-definition)
    - [Execution Model](#execution-model)
  - [6. Triggers, Events, and Await Event](#6-triggers-events-and-await-event)
    - [Triggers (`on:`)](#triggers-on)
    - [Events](#events)
    - [Await Event (`await_event`)](#await-event-await_event)
  - [7. Tool Registry \& Resolution](#7-tool-registry--resolution)
  - [8. Tool Manifest Schema](#8-tool-manifest-schema)
  - [9. API: CLI, HTTP, MCP](#9-api-cli-http-mcp)
  - [10. Security \& Secrets](#10-security--secrets)
    - [Example: Using Secrets](#example-using-secrets)
  - [11. Configuration Schema](#11-configuration-schema)
    - [Example: Memory (default)](#example-memory-default)
    - [Example: NATS](#example-nats)
      - [Configuration Schema (flow.config.schema.json)](#configuration-schema-flowconfigschemajson)
  - [12. Extensibility \& Integration Patterns](#12-extensibility--integration-patterns)
  - [13. Real-World Example Flows](#13-real-world-example-flows)
    - [Hello World](#hello-world)
    - [Fetch and Summarize](#fetch-and-summarize)
    - [Parallel LLMs (Fan-out and Combine)](#parallel-llms-fan-out-and-combine)
    - [Await/Resume (Human-in-the-Loop)](#awaitresume-human-in-the-loop)
    - [Human-in-the-Loop Approval (Airtable)](#human-in-the-loop-approval-airtable)
  - [14. Data Structures \& Schemas](#14-data-structures--schemas)
    - [Flow Schema (beemflow.schema.json)](#flow-schema-beemflowschemajson)
      - [Step Schema](#step-schema)
      - [Go Structs (model.go)](#go-structs-modelgo)
  - [15. Architecture \& Runtime](#15-architecture--runtime)
    - [Engine Execution Model](#engine-execution-model)
  - [16. Roadmap](#16-roadmap)
  - [17. License](#17-license)

---

## 1. What is BeemFlow?

BeemFlow is a **text-first, open protocol and runtime for AI-powered, event-driven automations**. It provides a protocol-agnostic, consistent interface for flows and tools—CLI, HTTP, and MCP clients all speak the same language. All tools (local, HTTP, MCP) are available in a single, LLM-native registry.

- **Write a single YAML file → run it locally, over REST, or through the Model Context Protocol (MCP).**
- **Flows are versioned, diffable, and LLM-friendly.**
- **Tools are protocol-agnostic and globally discoverable.**
- **AI-native: LLM prompts, event waits, and human-in-the-loop are first-class.**

---

## 2. Vision & Philosophy

| Legacy "No-Code" | **BeemFlow** |
|------------------|--------------|
| Drag-and-drop UIs that break at scale | **Plain-text YAML** — diff-able, version-controlled, LLM-parseable |
| Opaque SaaS black boxes | **Open runtime** + plug-in adapters |
| Human glue work | **LLM prompts are first-class** – AI is the default worker |
| Multiple brittle dashboards | **One spec → one run → one audit trail** |
| Vendor lock-in | **Protocol-agnostic**: CLI, REST, MCP, library |

BeemFlow is designed for:
- **LLM agents and co-workers**
- **Human-in-the-loop automations**
- **Composable, testable, and maintainable workflows**
- **Universal protocol: YAML, JSON, or native structs in any language**

---

## 3. Protocol Overview

A BeemFlow flow is defined in a single YAML file (or JSON, or native struct):

```yaml
name:       string                       # required
version:    string                       # optional semver
on:         list|object                  # triggers
vars:       map[string]                  # optional constants / secret refs
steps:      array of step objects        # required
catch:      array of step objects        # optional global error flow
```

- **Steps**: Each step is a tool call, logic, or wait.
- **Templating**: `{{ outputs.step.field }}`, `{{ vars.NAME }}`, helpers.
- **Parallelism**: `parallel: true` with nested `steps:`
- **Waits**: `await_event`, `wait`, durable and resumable
- **Registry**: Local, MCP, remote, GitHub—all tools in one namespace
- **API**: CLI, HTTP, MCP—same protocol, same flows

---

## 4. YAML/JSON/Struct Grammar

### YAML Example
```yaml
name: hello
on: cli.manual
steps:
  - id: greet
    use: core.echo
    with:
      text: "Hello, BeemFlow!"
  - id: print
    use: core.echo
    with:
      text: "{{ outputs.greet.text }}"
```

### JSON Example
```json
{
  "name": "hello",
  "on": "cli.manual",
  "steps": [
    { "id": "greet", "use": "core.echo", "with": { "text": "Hello, BeemFlow!" } },
    { "id": "print", "use": "core.echo", "with": { "text": "{{ outputs.greet.text }}" } }
  ]
}
```

### Go Struct Example
```go
flow := model.Flow{
  Name: "hello",
  On:   "cli.manual",
  Steps: []model.Step{
    {ID: "greet", Use: "core.echo", With: map[string]interface{}{"text": "Hello, BeemFlow!"}},
    {ID: "print", Use: "core.echo", With: map[string]interface{}{"text": "{{ outputs.greet.text }}"}},
  },
}
```

---

## 5. Flow Anatomy & Execution Model

### Step Definition
Each step in `steps:` supports:
```yaml
- id: string (required)
  use: string (tool identifier)
  with: object (tool inputs)
  if: expression (optional)
  foreach: expression (optional)
    as: string
    do: sequence
  parallel: true (optional, block-parallel only)
    steps: [ ... ]
  retry: { attempts: n, delay_sec: m } (optional)
  await_event: { source, match, timeout } (optional)
  wait: { seconds: n } | { until: ts } (optional)
  depends_on: [step ids] (optional)
```

- **Block-parallel**: `parallel: true` with nested `steps:`
- **Templating**: `{{ ... }}` for referencing event, vars, outputs, helpers
- **Error handling**: `catch:` block processes failures

### Execution Model
- Flows are executed step-by-step, supporting parallelism, waits, and event-driven pauses.
- Outputs from each step are available to subsequent steps via templating.
- Flows can pause on `await_event` and resume via event or callback.
- All state is persisted for durability and auditability.

---

## 6. Triggers, Events, and Await Event

### Triggers (`on:`)
A BeemFlow flow is started by one or more triggers, defined in the `on:` field at the top level of the YAML. The value can be a string, a list, or an object.

**Supported trigger types:**
- `cli.manual` — Manual trigger from the CLI or API.
- `event: <topic>` — Subscribes to a named event topic (e.g. `event: tweet.request`).
- `schedule.cron` — Runs on a cron schedule (requires a `cron:` field).
- `schedule.interval` — Runs on a fixed interval (requires an `every:` field).

### Events
- When a flow is triggered by an event, the event payload is available as `.event` in templates.
- For scheduled triggers, `.event` is usually empty unless injected by the runner.

### Await Event (`await_event`)
The `await_event` step pauses the flow until a matching event is received. This enables human-in-the-loop, webhook, or external event-driven automations.

**Schema:**
```yaml
- id: await_approval
  await_event:
    source: <string>         # e.g. "airtable", "bus", "slack"
    match:                   # map of fields to match on the incoming event
      <field>: <value>       # e.g. record_id: "{{ some_id }}", field: Status, equals: Approved
    timeout: <duration>      # (optional) e.g. "24h", "10m"
```

- The flow pauses at this step and resumes when a matching event arrives.
- The event that resumes the flow is available as `.event` in subsequent steps.

---

## 7. Tool Registry & Resolution

Tools are auto-discovered and prioritized as follows:
1. **Local manifests:** `.beemflow/registry.json` or `flow mcp install <registry>:<tool>`
2. **MCP servers:** `mcp://server/tool` (auto-discovered at runtime)
3. **Remote registries:** e.g. `https://hub.beemflow.com/index.json`
4. **GitHub shorthand:** `github:owner/repo[/path][@ref]`

**Registry Resolution Order:**
1. `$BeemFlow_REGISTRY` env var
2. `registry/index.json` (if exists)
3. Public hub at `https://hub.beemflow.com/index.json`

**Namespacing & Ambiguity:**
- All tool/server names can be qualified as `<registry>:<name>` (e.g., `smithery:airtable`).
- If ambiguous, user must specify the qualified name.
- CLI/API output always includes a `REGISTRY` column.

---

## 8. Tool Manifest Schema

Each tool is described by a JSON-Schema manifest:
```json
{
  "name": "tool.name",
  "description": "What this tool does",
  "kind": "task",
  "parameters": {
    "type": "object",
    "properties": {
      "input": { "type": "string", "default": "hello" }
    },
    "required": ["input"]
  },
  "endpoint": "https://..." // for HTTP tools
}
```

- **Manifest Default Injection**: BeemFlow injects any `default` values from the manifest's parameters into the request body for missing fields. This keeps flows DRY and ergonomic.

---

## 9. API: CLI, HTTP, MCP

BeemFlow exposes a consistent interface for all operations:

| Operation         | CLI Command                  | HTTP Endpoint                | MCP Tool Name      |
|-------------------|-----------------------------|------------------------------|--------------------|
| List flows        | `flow list`                  | `GET /runs`                  | `listFlows`        |
| Get flow          | `flow get <name>`            | (not exposed)                | `getFlow`          |
| Validate flow     | `flow lint <file>`           | `POST /validate`             | `validateFlow`     |
| Run flow          | `flow run <name> [--event]`  | `POST /runs`                 | `startRun`         |
| Get run status    | `flow status <run_id>`       | `GET /runs/{id}`             | `getRun`           |
| Resume run        | `flow resume <token>`        | `POST /resume/{token}`       | (not exposed)      |
| Test flow         | `flow test <file>`           | `POST /test`                 | (not exposed)      |
| Graph flow        | `flow graph <file>`          | `GET /graph`                 | `graphFlow`        |
| List tools        | `flow tools`                 | `GET /tools`                 | (not exposed)      |
| Get tool manifest | (n/a)                        | `GET /tools/{name}`          | (not exposed)      |
| Inline run        | (n/a)                        | `POST /runs/inline`          | `flow.execute`     |
| Metadata          | (n/a)                        | `GET /metadata`              | `describe`         |

All endpoints accept/return JSON.

---

## 10. Security & Secrets

- Secrets can be injected from env, event, or secrets backend.
- HMAC signatures for resume callbacks.
- Step-level timeouts and resource limits (roadmap).

### Example: Using Secrets
```yaml
steps:
  - id: notify_ops
    use: slack.chat.postMessage
    with:
      channel: "#ops"
      text:    "All systems go!"
      token:   "{{ secrets.SLACK_TOKEN }}"
```

---

## 11. Configuration Schema

BeemFlow is configured via a JSON file. All fields in `config.Config` are supported. See `flow.config.schema.json` for the full schema.

### Example: Memory (default)
```json
{
  "storage": { "driver": "sqlite", "dsn": "beemflow.db" }
}
```

### Example: NATS
```json
{
  "event": {
    "driver": "nats",
    "url": "nats://user:pass@your-nats-host:4222"
  }
}
```

#### Configuration Schema (flow.config.schema.json)
```json
{
  "storage": { "driver": "string", "dsn": "string" },
  "event": { "driver": "string", "url": "string" },
  "blob": { "driver": "string", "bucket": "string" },
  "secrets": { "driver": "string", "region": "string", "prefix": "string" },
  "registries": [ { "type": "string", "url": "string", "path": "string" } ],
  "http": { "host": "string", "port": "integer" },
  "log": { "level": "string" },
  "flowsDir": "string",
  "mcpServers": { "command": "string", "args": ["string"], ... }
}
```

---

## 12. Extensibility & Integration Patterns

- **Add a local tool:** `flow mcp install <registry>:<tool>` or add entries to `.beemflow/registry.json`.
- **Add an MCP server:** `flow mcp install <registry>:<server>` or configure in `.beemflow/registry.json`.
- **Add a remote tool:** Reference a remote registry or GitHub manifest.
- **Write a custom adapter:** Implement the Adapter interface in Go.
- **Extend Event Bus:** Add fields to `event` config (e.g. `clusterID`, `clientID`, TLS options) and wire them into `NewEventBusFromConfig`.
- **Environment Overrides:** (Future) Support env vars like `BeemFlow_EVENT_DRIVER` to override config at runtime.

---

## 13. Real-World Example Flows

### Hello World
```yaml
name: hello
on: cli.manual
steps:
  - id: greet
    use: core.echo
    with:
      text: "Hello, world, I'm BeemFlow!"
  - id: greet_again
    use: core.echo
    with:
      text: "Aaand once more: {{ greet.text }}"
```

### Fetch and Summarize
```yaml
name: fetch_and_summarize
on: cli.manual
vars:
  fetch_url: "https://en.wikipedia.org/wiki/Artificial_intelligence"
steps:
  - id: fetch_page
    use: http.fetch
    with:
      url: "{{ fetch_url }}"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following web page in 3 bullets."
        - role: user
          content: "{{ fetch_page.body }}"
  - id: print
    use: core.echo
    with:
      text: "Summary: {{ summarize.choices.0.message.content }}"
```

### Parallel LLMs (Fan-out and Combine)
```yaml
name: parallel_openai_nested
on: cli.manual
vars:
  prompt1: "Generate a fun fact about space"
  prompt2: "Generate a fun fact about oceans"
steps:
  - id: fanout
    parallel: true
    steps:
      - id: chat1
        use: openai.chat_completion
        with:
          model: "gpt-3.5-turbo"
          messages:
            - role: user
              content: "{{ prompt1 }}"
      - id: chat2
        use: openai.chat_completion
        with:
          model: "gpt-3.5-turbo"
          messages:
            - role: user
              content: "{{ prompt2 }}"
  - id: combine
    depends_on: [fanout]
    use: core.echo
    with:
      text: |
        Combined responses:
        - chat1: {{ chat1.choices.0.message.content }}
        - chat2: {{ chat2.choices.0.message.content }}
```

### Await/Resume (Human-in-the-Loop)
```yaml
name: echo_await_resume
on:
  - event: test.manual
vars:
  token: "abc123"
steps:
  - id: echo_start
    use: core.echo
    with:
      text: "Started (token: {{ token }})"
  - id: wait_for_resume
    await_event:
      source: test
      match:
        token: "{{ token }}"
      timeout: 1h
  - id: echo_resumed
    use: core.echo
    with:
      text: "Resumed with: {{ event.resume_value }} (token: {{ token }})"
```

### Human-in-the-Loop Approval (Airtable)
```yaml
name: tweet_copy_approval
on:
  - event: tweet.request
steps:
  - id: generate_copy
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Generate a tweet about {{event.topic}}"
  - id: create_airtable_record
    use: mcp://airtable/create_record
    with:
      baseId: "{{ secrets.AIRTABLE_BASE_ID }}"
      tableId: "{{ secrets.AIRTABLE_TABLE_ID }}"
      fields:
        Copy: "{{ generate_copy.choices.0.message.content }}"
        Status: "Pending"
  - id: await_approval
    await_event:
      source: airtable
      match:
        record_id: "{{ create_airtable_record.id }}"
        field: Status
        equals: Approved
      timeout: 24h
  - id: mark_posted
    use: mcp://airtable/update_records
    with:
      baseId: "{{ secrets.AIRTABLE_BASE_ID }}"
      tableId: "{{ secrets.AIRTABLE_TABLE_ID }}"
      records:
        - recordId: "{{ create_airtable_record.id }}"
          fields:
            Status: "Posted"
```

---

## 14. Data Structures & Schemas

### Flow Schema (beemflow.schema.json)
```json
{
  "name": "string",
  "version": "string",
  "on": {},
  "vars": { "type": "object" },
  "steps": [ { ...step... } ],
  "catch": [ { ...step... } ],
  "mcpServers": { ... }
}
```

#### Step Schema
```json
{
  "id": "string",
  "use": "string",
  "with": { "type": "object" },
  "depends_on": ["string"],
  "parallel": "boolean",
  "if": "string",
  "foreach": "string",
  "as": "string",
  "do": [ { ...step... } ],
  "retry": { "attempts": "integer", "delay_sec": "integer" },
  "await_event": { "source": "string", "match": { ... }, "timeout": "string" },
  "wait": { "seconds": "integer", "until": "string" },
  "steps": [ { ...step... } ]
}
```

#### Go Structs (model.go)
```go
type Flow struct {
  Name    string
  Version string
  On      interface{}
  Vars    map[string]interface{}
  Steps   []Step
  Catch   []Step
}

type Step struct {
  ID         string
  Use        string
  With       map[string]interface{}
  DependsOn  []string
  Parallel   bool
  If         string
  Foreach    string
  As         string
  Do         []Step
  Steps      []Step
  Retry      *RetrySpec
  AwaitEvent *AwaitEventSpec
  Wait       *WaitSpec
}
```

---

## 15. Architecture & Runtime

- **Router & planner** (DAG builder)
- **Executor** (persistent state, retries, awaits)
- **Event bus** (memory, NATS, Temporal future)
- **Registry & adapters** (tool discovery, MCP, HTTP, custom)
- **Storage** (SQLite, Postgres, in-memory)
- **Blob store** (optional, for file outputs)

### Engine Execution Model
- Flows are parsed and validated against the schema.
- Steps are executed in order, supporting parallelism and event waits.
- State is persisted for durability and auditability.
- Awaited events pause the flow and resume on event/callback.
- Outputs are available for downstream steps and for audit.

---

## 16. Roadmap

- VS Code extension (YAML + Mermaid preview)
- Flow template gallery (`flow init payroll` etc.)
- Cron & Temporal adapters
- Hot-reload adapters without downtime
- On-chain event bus (experimental)
- Step-level resource limits and advanced event bus drivers
- Protobuf-based protocol spec and generated language bindings

---

## 17. License

MIT — use it, remix it, ship it. Commercial cloud & SLA on the way.

---

> Docs at <https://docs.beemflow.com> • X: [@BeemFlow](https://X.com/beemflow) 