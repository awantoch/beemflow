# BeemFlow Protocol & Specification

---

This document is the **canonical, LLM-ingestible specification** for BeemFlow. It is fully self-contained: all YAML grammar, config, API, and extension patterns are included below.

---

## 1. Purpose & Vision

BeemFlow is a text-first, open protocol and runtime for AI-powered, event-driven automations. It provides a **protocol-agnostic, consistent interface** for flows and tools—CLI, HTTP, and MCP clients all speak the same language. All tools (local, HTTP, MCP) are available in a single, LLM-native registry.

---

## 2. YAML File Grammar

A BeemFlow flow is defined in a single YAML file:

```yaml
name:       string                       # required
version:    string                       # optional semver
on:         list|object                  # triggers
vars:       map[string]                  # optional constants / secret refs
steps:      array of step objects        # required
catch:      array of step objects        # optional global error flow (ordered list)
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

---

## 3. Step Definition Keys

Each step in `steps:` supports the following keys:

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
1. **Local manifests**: `tools/<name>.json`
2. **MCP servers**: `mcp://server/tool` (auto-discovered at runtime)
3. **Remote registries**: e.g. `https://hub.beemflow.com/index.json`
4. **GitHub shorthand**: `github:owner/repo[/path][@ref]`

All tools are exposed in a single, LLM-native registry for use in flows, CLI, HTTP, MCP, or LLMs.

**Registry Resolution Order:**
1. If the `BEEMFLOW_REGISTRY` environment variable is set, it is used.
2. If `registry/index.json` exists, it is used.
3. Otherwise, the public hub at `https://hub.beemflow.com/index.json` is used.

### 4a. Registry Namespacing & Ambiguity

- When multiple registries are enabled, all tool/server names are qualified as `<registry>:<name>` (e.g., `smithery:airtable`, `local:mytool`).
- CLI and API output always includes a `REGISTRY` column.
- If a name is ambiguous (exists in more than one registry), the user must specify the qualified name.
- If only one match exists, the unqualified name is accepted for convenience.

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

**Request:**
```http
POST /runs
Content-Type: application/json

{
  "flow": "hello",
  "event": {}
}
```
**Response:**
```json
{
  "run_id": "b1e2...",
  "status": "STARTED"
}
```

### Example: Get Run Status (HTTP)

**Request:**
```http
GET /runs/b1e2...
```
**Response:**
```json
{
  "id": "b1e2...",
  "flow_name": "hello",
  "status": "SUCCEEDED",
  "outputs": { "print": { "text": "Hello, BeemFlow!" } }
}
```

### Example: List Tools (HTTP)

**Request:**
```http
GET /tools
```
**Response:**
```json
[
  { "name": "core.echo", "description": "Echo text", ... },
  { "name": "http.fetch", "description": "Fetch a URL", ... }
]
```

### Example: Graph Flow (HTTP)

**Request:**
```http
GET /graph?flow=hello
```
**Response:**
```mermaid
%% Mermaid graph
flowchart TD
  greet --> print
```

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
BeemFlow automatically injects any `default` values from the manifest's parameters into the request body for missing fields. This means you can omit defaulted fields in your YAML flows, and the runtime will fill them in, making flows DRY and ergonomic.

---

## 7. Configuration (flow.config.json)

The runtime is configured via a JSON file. All fields in `config.Config` are supported. See [flow_config.schema.json](flow_config.schema.json) for the full schema.

### Example Configuration

#### Memory (default)
```jsonc
{
  "storage": { "driver": "sqlite", "dsn": "beemflow.db" }
  // no "event" block → in-mem bus
}
```

#### NATS
```jsonc
{
  "event": {
    "driver": "nats",
    "url": "nats://user:pass@your-nats-host:4222"
  }
}
```

> **Event Bus**
> • driver=`memory` (default, in-process)
> • driver=`nats` (requires `url`)
> • unknown drivers error out

BeemFlow always loads the built-in curated registry and Smithery (if SMITHERY_API_KEY is set); you don't need to specify these in your config.

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

HTTP, OpenAI, MCP, and custom adapters are all supported.

---

## 9. Durable Waits & Callbacks

Flows can pause on `await_event` and resume via `POST /resume/{token}` (HMAC-signed). State is persisted in the configured storage backend.

### Example Await Event Step

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
      token:   "{{secrets.SLACK_TOKEN}}"
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
    use: openai
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
      text: "{{.outputs.summarize.choices[0].message.content}}"
```

---

## 12. Extensibility Patterns

- **Add a local tool:** Drop a manifest in `tools/`.
- **Add an MCP server:** Add config in `mcp_servers/` and reference in config.
- **Add a remote tool:** Reference a remote registry or GitHub manifest.
- **Write a custom adapter:** Implement the Adapter interface in Go.
- **Extend Event Bus:** Add fields to `event` config (e.g. `clusterID`, `clientID`, TLS options) and wire them into `NewEventBusFromConfig`.
- **Environment Overrides:** Future: support env vars like `BEEMFLOW_EVENT_DRIVER` to override config at runtime.

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
