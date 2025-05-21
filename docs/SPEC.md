# BeemFlow LLM Agent Quick Start

> **How to use BeemFlow as an LLM agent:**
>
> 1. **List Capabilities:** Call `describe` to see all available tools and commands.
> 2. **Fetch the Protocol Spec:** Call `spec` to get the full YAML grammar, config, and API reference.
> 3. **Act on User Intent:**
>    - If the user asks for a new flow, generate a complete YAML using the spec and template below.
>    - If the user wants to run or list flows, use `listFlows`, `startRun`, etc.
>    - Always show the YAML or result, and ask if the user wants to install/run/customize it.
> 4. **Don't ask for permission to fetch the spec or describe—just do it.**

---

## Example MCP Tool Metadata

```json
[
  { "id": "spec", "type": "mcp", "description": "BeemFlow Protocol & Specification" },
  { "id": "listFlows", "type": "mcp", "description": "List all flows" },
  { "id": "startRun", "type": "mcp", "description": "Start a new run" },
  ...
]
```

---

## YAML Flow Template

```yaml
name: <flow-name>
on: cli.manual
vars:
  URL: "https://example.com"
steps:
  - id: fetch
    use: http.fetch
    with:
      url: "{{.vars.URL}}"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following text."
        - role: user
          content: "{{.outputs.fetch.body}}"
```

---

## Next-Step Rubric for LLMs

> **When the user asks for a flow:**
> - Write a complete YAML.
> - Don't ask for more info unless absolutely necessary.
> - Always include a `vars:` block if the flow uses variables.

---

# Real-World BeemFlow YAML Examples

### 1. Hello World
A minimal flow that prints a message.
```yaml
name: hello
on: cli.manual
steps:
  - id: greet
    use: core.echo
    with:
      text: "Hello, BeemFlow!"
```

### 2. Fetch and Summarize a URL
Fetches a web page and summarizes it with GPT-4o.
```yaml
name: fetch_and_summarize
on: cli.manual
vars:
  URL: "https://en.wikipedia.org/wiki/Artificial_intelligence"
steps:
  - id: fetch
    use: http.fetch
    with:
      url: "{{.vars.URL}}"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following text in 3 bullet points."
        - role: user
          content: "{{.outputs.fetch.body}}"
  - id: print
    use: core.echo
    with:
      text: "{{ summarize.choices.0.message.content }}"
```

### 3. Slack Notification
Sends a message to a Slack channel using a secret token.
```yaml
name: notify_ops
on: cli.manual
steps:
  - id: notify
    use: slack.chat.postMessage
    with:
      channel: "#ops"
      text: "All systems go!"
      token: "{{.secrets.SLACK_TOKEN}}"
```

### 4. Parallel Steps
Runs two steps in parallel.
```yaml
name: parallel_example
on: cli.manual
steps:
  - id: parallel_block
    parallel: true
    steps:
      - id: step1
        use: core.echo
        with:
          text: "This runs in parallel (1)"
      - id: step2
        use: core.echo
        with:
          text: "This runs in parallel (2)"
```

### 5. Await Event (Human-in-the-Loop)
Waits for an external approval before continuing.
```yaml
name: await_approval
on: cli.manual
steps:
  - id: await_approval
    await_event:
      source: airtable
      match:
        record_id: "{{airtable_row.id}}"
        field: Status
        equals: Approved
      timeout: 24h
  - id: notify
    use: core.echo
    with:
      text: "Approval received!"
```

---

# Triggers, Events, and Await Event

## Triggers (`on:`)
A BeemFlow flow is started by one or more triggers, defined in the `on:` field at the top level of the YAML. The value can be a string, a list, or an object.

**Supported trigger types:**

- `cli.manual` — Manual trigger from the CLI or API.
- `event: <topic>` — Subscribes to a named event topic (e.g. `event: tweet.request`).
- `schedule.cron` — Runs on a cron schedule (requires a `cron:` field).
- `schedule.interval` — Runs on a fixed interval (requires an `every:` field).

**Examples:**

```yaml
on: cli.manual
```

```yaml
on:
  - event: tweet.request
  - schedule.cron
cron: "0 9 * * 1-5"  # every weekday at 09:00
```

```yaml
on:
  - schedule.interval
every: "1h"
```

---

## Events
- When a flow is triggered by an event (e.g. `event: tweet.request`), the event payload is available as `.event` in templates.
- For scheduled triggers, `.event` is usually empty unless injected by the runner.
- You can use event fields in step inputs, conditions, and templates.

**Example:**

```yaml
steps:
  - id: greet
    use: core.echo
    with:
      text: "Hello, {{.event.user}}!"
```

---

## Await Event (`await_event`)
The `await_event` step pauses the flow until a matching event is received. This enables human-in-the-loop, webhook, or external event-driven automations.

**Schema:**

```yaml
- id: await_approval
  await_event:
    source: <string>         # e.g. "airtable", "bus", "slack"
    match:                   # map of fields to match on the incoming event
      <field>: <value>       # e.g. record_id: "{{some_id}}", field: Status, equals: Approved
    timeout: <duration>      # (optional) e.g. "24h", "10m"
```

**How it works:**
- The flow pauses at this step.
- BeemFlow subscribes to events from the given `source`.
- When an event arrives that matches all fields in `match`, the flow resumes.
- If `timeout` is set and no event arrives in time, the flow can error or take a catch path.

**Example:**

```yaml
- id: await_approval
  await_event:
    source: airtable
    match:
      record_id: "{{.outputs.create_airtable_record.id}}"
      field: Status
      equals: Approved
    timeout: 24h
```

**Notes:**
- The `match` map is used to filter incoming events. All fields must match for the step to resume.
- The event that resumes the flow is available as `.event` in subsequent steps.
- The `source` determines which event bus or integration to listen on (e.g. `airtable`, `bus`, `slack`).

---

## Advanced: Custom Event Topics
You can define custom event topics and trigger flows on them:

```yaml
on:
  - event: my.custom.topic
```

And publish events to those topics from other flows or external systems.

---

## Example: Full Await Event Flow

```yaml
name: approval_flow
on: event: approval.requested

steps:
  - id: await_approval
    await_event:
      source: bus
      match:
        request_id: "{{.event.request_id}}"
        status: approved
      timeout: 48h
  - id: notify
    use: core.echo
    with:
      text: "Approval received for request {{.event.request_id}}!"
```

---

# BeemFlow Protocol & Specification

---

> **The canonical, LLM-ingestible spec for BeemFlow.**
> All YAML grammar, config, API, and extension patterns in one place.

---

## Quick Reference (Cheat Sheet)

- **Flow file:** Single YAML, versioned, text-first, LLM-friendly
- **Triggers:** `on: cli.manual`, `on: schedule.cron`, etc.
- **Steps:** Each step = tool call, logic, or wait
- **Templating:** `{{.outputs.step.field}}`, `{{.vars.NAME}}`, helpers
- **Parallelism:** `parallel: true` with nested `steps:`
- **Waits:** `await_event`, `wait`, durable and resumable
- **Registry:** Local, MCP, remote, GitHub—all tools in one namespace
- **API:** CLI, HTTP, MCP—same protocol, same flows

---

## 1. Purpose & Vision

BeemFlow is a text-first, open protocol and runtime for AI-powered, event-driven automations. It provides a **protocol-agnostic, consistent interface** for flows and tools—CLI, HTTP, and MCP clients all speak the same language. All tools (local, HTTP, MCP) are available in a single, LLM-native registry.

---

## 2. YAML Flow Grammar

A BeemFlow flow is defined in a single YAML file:

```yaml
name:       string                       # required
version:    string                       # optional semver
on:         list|object                  # triggers
vars:       map[string]                  # optional constants / secret refs
steps:      array of step objects        # required
catch:      array of step objects        # optional global error flow
```

### Example Flow

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
      text: "{{.outputs.greet.text}}"
```

**Why it's powerful:**
- All logic is in YAML—versioned, diffable, LLM-friendly.
- Steps can reference outputs, vars, secrets, and helpers.

---

## 3. Step Definition

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

- Only block-parallel (`parallel: true` with nested `steps:`) is supported.
- Templating: `{{ ... }}` for referencing event, vars, outputs, helpers.

---

## 4. Tool Registry & Resolution

Tools are auto-discovered and prioritized as follows:
1. **Local manifests:** `.beemflow/registry.json` or `flow mcp install <registry>:<tool>`
2. **MCP servers:** `mcp://server/tool` (auto-discovered at runtime)
3. **Remote registries:** e.g. `https://hub.beemflow.com/index.json`
4. **GitHub shorthand:** `github:owner/repo[/path][@ref]`

**Registry Resolution Order:**
1. `$BEEMFLOW_REGISTRY` env var
2. `registry/index.json` (if exists)
3. Public hub at `https://hub.beemflow.com/index.json`

**Namespacing & Ambiguity:**
- All tool/server names can be qualified as `<registry>:<name>` (e.g., `smithery:airtable`).
- If ambiguous, user must specify the qualified name.
- CLI/API output always includes a `REGISTRY` column.

---

## 5. Protocol-Agnostic API (CLI, HTTP, MCP)

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
| Assistant         | (n/a)                        | `POST /assistant/chat`       | `beemflow.assistant`|
| Metadata          | (n/a)                        | `GET /metadata`              | `describe`         |

All endpoints accept/return JSON.

### Example: Run a Flow (HTTP)

```http
POST /runs
Content-Type: application/json

{
  "flow": "hello",
  "event": {}
}
```
**What happens?**
- Starts a new run of the `hello` flow. Returns a run ID and status.

---

## 6. Tool Manifest Schema (JSON-Schema)

Each tool is described by a JSON-Schema manifest:

```jsonc
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

**Manifest Default Injection:**
- BeemFlow injects any `default` values from the manifest's parameters into the request body for missing fields. This keeps flows DRY and ergonomic.

---

## 7. Configuration (`flow.config.json`)

The runtime is configured via a JSON file. All fields in `config.Config` are supported. See [flow.config.schema.json](flow.config.schema.json) for the full schema.

### Example: Memory (default)
```jsonc
{
  "storage": { "driver": "sqlite", "dsn": "beemflow.db" }
}
```

### Example: NATS
```jsonc
{
  "event": {
    "driver": "nats",
    "url": "nats://user:pass@your-nats-host:4222"
  }
}
```

> **Event Bus:**
> - `driver: memory` (default, in-process)
> - `driver: nats` (requires `url`)
> - Unknown drivers error out

BeemFlow always loads the built-in curated registry and Smithery (if `SMITHERY_API_KEY` is set); you don't need to specify these in your config.

---

## 8. Adapter Interfaces

All tool integrations implement the `Adapter` interface:

```go
type Adapter interface {
  ID() string
  Execute(ctx context.Context, inputs map[string]any) (map[string]any, error)
  Manifest() *ToolManifest
}
```

- HTTP, OpenAI, MCP, and custom adapters are all supported.

---

## 9. Durable Waits & Callbacks

Flows can pause on `await_event` and resume via `POST /resume/{token}` (HMAC-signed). State is persisted in the configured storage backend.

### Example: Await Event Step

```yaml
- id: await_approval
  await_event:
    source: airtable
    match:
      record_id: "{{airtable_row.id}}"
      field: Status
      equals: Approved
    timeout: 24h
- id: notify
  use: core.echo
  with:
    text: "Approval received!"
```

**Why it's powerful:**
- Enables human-in-the-loop, webhook, or external event-driven automations.
- Flows are durable and resumable.

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
      token:   "{{.secrets.SLACK_TOKEN}}"
```

---

## 11. Canonical Example Flows

### Hello World

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
      text: "{{.outputs.greet.text}}"
```

### Fetch and Summarize

```yaml
name: fetch_and_summarize
on: cli.manual
steps:
  - id: fetch
    use: http.fetch
    with:
      url: "https://en.wikipedia.org/api/rest_v1/page/summary/Artificial_intelligence"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following text in 3 bullet points."
        - role: user
          content: "{{.outputs.fetch.body}}"
  - id: print
    use: core.echo
    with:
      text: "{{ summarize.choices.0.message.content }}"
```

---

## 12. Extensibility Patterns

- **Add a local tool:** `flow mcp install <registry>:<tool>` or add entries to `.beemflow/registry.json`.
- **Add an MCP server:** `flow mcp install <registry>:<server>` or configure in `.beemflow/registry.json`.
- **Add a remote tool:** Reference a remote registry or GitHub manifest.
- **Write a custom adapter:** Implement the Adapter interface in Go.
- **Extend Event Bus:** Add fields to `event` config (e.g. `clusterID`, `clientID`, TLS options) and wire them into `NewEventBusFromConfig`.
- **Environment Overrides:** (Future) Support env vars like `BEEMFLOW_EVENT_DRIVER` to override config at runtime.

---

## 13. System Prompt(s) for LLMs

LLMs can use the following system prompt to interact with BeemFlow as a tool registry and flow orchestrator:

> "You are an expert BeemFlow agent. You can list, validate, and run flows, discover and call tools, and manage automations using the BeemFlow protocol. All tools and flows are described in the registry. Always use the protocol-agnostic API and follow the canonical YAML grammar."

---

## 14. License

MIT. Use it, remix it, ship it.

---

## 15. Roadmap

- VSCode extension (syntax highlight, lint, live Mermaid)
- Optional web canvas (edits YAML)
- Flow template gallery (`flow init`)
- Temporal backend adapter
- Metrics / observability plugin
- Cron triggers, step-level resource limits, advanced event bus drivers, and adapter hot-reload (roadmap)

---

## 16. Integration Patterns

- MCP, HTTP, and custom adapters are all supported and interoperable.

---

**Next Steps:**
- See [README.md](../README.md) for real-world examples and onboarding.
- Try writing your own flow or tool manifest.
- Join the community: [Discord](https://discord.gg/beemflow)
