# BeemFlow LLM Agent Quick Start

> **How to use BeemFlow as an LLM agent:**
>
> 1. **Fetch the Protocol Spec:** Call `spec` to get the full YAML grammar, config, and API reference.
> 2. **Act on User Intent:**
>    - If the user asks for a new flow, generate a complete YAML using the spec and template below.
>    - If the user wants to run or list flows, use `listFlows`, `startRun`, etc.
>    - Always show the YAML or result, and ask if the user wants to install/run/customize it.
> 3. **Don't ask for permission to fetch the spec or describe—just do it.**

---

## Example MCP Tool Metadata

```json
[
  { "id": "beemflow_spec", "type": "mcp", "description": "BeemFlow Protocol & Specification" },
  { "id": "beemflow_list_flows", "type": "mcp", "description": "List all flows" },
  { "id": "beemflow_start_run", "type": "mcp", "description": "Start a new run" },
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
      url: "https://api.example.com/data"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following text."
        - role: user
          content: "{{ outputs.fetch.body }}"
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
      url: "{{ vars.URL }}"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following text in 3 bullet points."
        - role: user
          content: "{{ outputs.fetch.body }}"
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
      token: "{{ secrets.SLACK_TOKEN }}"
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
        record_id: "{{ airtable_row.id }}"
        field: Status
        equals: Approved
      timeout: 24h
  - id: notify
    use: core.echo
    with:
      text: "Approval received!"
```

---

# HTTP & API Integration: Three Powerful Patterns

> **For LLMs:** BeemFlow provides three complementary ways to integrate with HTTP APIs and external services. Choose the right pattern based on the use case complexity and requirements.

## Pattern Selection Guide for LLMs

| **Use Case** | **Pattern** | **Tool/Syntax** | **When to Use** |
|--------------|-------------|-----------------|-----------------|
| Simple web page fetch | Registry Tool | `http.fetch` | Getting started, simple GET requests |
| OpenAI/Anthropic calls | Registry Tool | `openai.chat_completion` | AI services with smart defaults |
| Custom REST API (simple) | Registry Tool | Create JSON manifest | Basic API calls, simple reuse |
| Custom REST API (advanced) | MCP Server | `mcp://my-api/search` | Caching, retries, business logic |
| Database operations | MCP Server | `mcp://postgres/query` | Stateful connections, complex logic |
| File processing | MCP Server | `mcp://filesystem/read` | File system operations |
| One-off webhook/custom request | Generic HTTP | `http` with POST | Single-use, non-reusable requests |

---

## 🟢 Pattern 1: Registry Tools (Recommended Default)

**Best for:** Simple APIs, getting started, common services

**Key characteristics:**
- **Zero configuration** - just provide required parameters
- **Pre-configured** with endpoints, headers, and validation
- **Battle-tested** - work out of the box
- **API-specific** - each tool knows service quirks

### Examples:

#### Simple HTTP Fetching
```yaml
- id: fetch_page
  use: http.fetch
  with:
    url: "https://api.example.com/data"
```

#### AI Services with Smart Defaults
```yaml
- id: chat
  use: openai.chat_completion
  with:
    model: "gpt-4o"
    messages:
      - role: user
        content: "Hello, world!"

- id: anthropic_chat
  use: anthropic.chat_completion
  with:
    model: "claude-3-haiku-20240307"
    messages:
      - role: user
        content: "Hello, Claude!"
```

#### Other Common Registry Tools
```yaml
# Slack messaging
- id: notify_team
  use: slack.chat.postMessage
  with:
    channel: "#general"
    text: "Deployment complete!"
    token: "{{ secrets.SLACK_TOKEN }}"

# Email sending
- id: send_email
  use: postmark.email.send
  with:
    to: "user@example.com"
    subject: "Welcome!"
    template: "welcome_template"
```

---

## 🔧 Pattern 2: Generic HTTP Adapter (Maximum Flexibility)

**Best for:** Complex APIs, custom authentication, non-standard requests

**Key characteristics:**
- **Complete HTTP control** - any method, headers, body
- **No assumptions** - you specify exactly what gets sent
- **Perfect for** - REST APIs, webhooks, custom protocols
- **Raw power** - handles any HTTP scenario

### Examples:

#### Full HTTP Control
```yaml
- id: api_call
  use: http
  with:
    url: "https://api.example.com/data"
    method: "POST"
    headers:
      Authorization: "Bearer {{ secrets.API_KEY }}"
      Content-Type: "application/json"
      X-Custom-Header: "my-value"
    body: |
      {
        "query": "{{ user_input }}",
        "options": {
          "format": "json",
          "limit": 100
        }
      }
```

#### Different HTTP Methods
```yaml
# GET with custom headers
- id: get_with_auth
  use: http
  with:
    url: "https://api.example.com/protected"
    method: "GET"
    headers:
      Authorization: "Bearer {{ secrets.TOKEN }}"

# PUT request
- id: update_resource
  use: http
  with:
    url: "https://api.example.com/resource/123"
    method: "PUT"
    headers:
      Content-Type: "application/json"
    body: '{"status": "updated"}'

# DELETE request
- id: delete_resource
  use: http
  with:
    url: "https://api.example.com/resource/123"
    method: "DELETE"
    headers:
      Authorization: "Bearer {{ secrets.TOKEN }}"
```

#### Webhook Integration
```yaml
- id: send_webhook
  use: http
  with:
    url: "{{ webhook_url }}"
    method: "POST"
    headers:
      Content-Type: "application/json"
      X-Webhook-Signature: "{{ secrets.WEBHOOK_SECRET }}"
    body: |
      {
        "event": "flow_completed",
        "data": {
          "flow_id": "{{ flow.name }}",
          "timestamp": "{{ now }}",
          "results": {{ outputs | json }}
        }
      }
```

---

## 🚀 Pattern 3: MCP Servers (Complex Integrations)

**Best for:** Databases, file systems, stateful services, complex workflows

**Key characteristics:**
- **Stateful connections** - maintain database connections, file handles
- **Rich protocols** - beyond HTTP, supports any communication pattern
- **Ecosystem** - thousands of MCP servers available
- **Complex logic** - servers can implement sophisticated business logic

### Examples:

#### Database Operations
```yaml
# PostgreSQL queries
- id: query_users
  use: mcp://postgres/query
  with:
    sql: "SELECT * FROM users WHERE active = true"
    params: []

# Database transactions
- id: update_user
  use: mcp://postgres/transaction
  with:
    queries:
      - sql: "UPDATE users SET last_login = NOW() WHERE id = ?"
        params: ["{{ user_id }}"]
      - sql: "INSERT INTO login_log (user_id, timestamp) VALUES (?, NOW())"
        params: ["{{ user_id }}"]
```

#### File System Operations
```yaml
# Read files
- id: read_config
  use: mcp://filesystem/read
  with:
    path: "/etc/app/config.json"

# Process multiple files
- id: process_reports
  use: mcp://filesystem/glob
  with:
    pattern: "/data/reports/*.csv"
    action: "read"
```

#### Complex Business Logic
```yaml
# Custom business logic server
- id: calculate_pricing
  use: mcp://pricing-engine/calculate
  with:
    product_id: "{{ product.id }}"
    customer_tier: "{{ customer.tier }}"
    quantity: "{{ order.quantity }}"
    market_conditions: "{{ market.data }}"
```

---

## Creating Custom Registry Tools

**The smart way to handle custom APIs:** Define once as a JSON manifest, reuse everywhere.

You can create reusable tools by adding them to `.beemflow/registry.json`:

```json
{
  "type": "tool",
  "name": "my_api.search",
  "description": "Search my custom API",
  "parameters": {
    "type": "object",
    "required": ["query"],
    "properties": {
      "query": {"type": "string", "description": "Search query"},
      "limit": {"type": "integer", "default": 10, "description": "Max results"}
    }
  },
  "endpoint": "https://my-api.com/search",
  "method": "POST",
  "headers": {
    "Authorization": "Bearer $env:MY_API_KEY",
    "Content-Type": "application/json"
  }
}
```

Then use it simply across all your flows:
```yaml
- id: search
  use: my_api.search
  with:
    query: "{{ user_input }}"
    # limit defaults to 10 from manifest
```

**Benefits over repeating `http` configurations:**
- **DRY principle** - Define once, use everywhere
- **Type safety** - Parameter validation and defaults  
- **Maintainability** - Update API config in one place
- **Team sharing** - Colleagues can discover and use your APIs
- **Documentation** - Built-in descriptions and examples

### **JSON Manifests vs MCP Servers for Custom APIs**

**Start with JSON manifests** for simple API integrations:
```yaml
- id: search
  use: my_api.search  # Simple JSON manifest
  with:
    query: "{{ user_input }}"
```

**Upgrade to MCP servers** when you need advanced features:
```yaml
- id: search
  use: mcp://my-api/search  # Advanced MCP server
  with:
    query: "{{ user_input }}"
    # Server handles caching, retries, rate limiting
```

**Choose MCP servers when you need:**
- **Caching** - Store API responses to reduce calls
- **Rate limiting** - Handle API quotas intelligently
- **Retries & circuit breakers** - Robust error handling
- **Data transformation** - Complex response processing
- **Stateful operations** - Maintain connections or sessions
- **Business logic** - Custom validation, enrichment, workflows
- **Multiple endpoints** - Expose many related API operations

**The progression:**
1. **Start simple** - JSON manifest for basic API calls
2. **Add complexity** - Upgrade to MCP server when needed
3. **Share & scale** - Publish your MCP server for others

---

## LLM Guidelines for HTTP Pattern Selection

### When generating flows for users:

1. **Default to Registry Tools** (`http.fetch`, `openai.chat_completion`) for common operations
2. **Create JSON manifests** for custom APIs that will be reused across flows
3. **Use Generic HTTP** (`http`) only for:
   - One-off requests that won't be reused
   - Quick prototyping before creating a manifest
   - Webhook endpoints with dynamic URLs
4. **Suggest MCP Servers** (`mcp://`) for:
   - Database operations
   - File system access
   - Stateful services
   - Complex business logic

### Example Decision Tree:
```
User wants to: "Call an API"
├─ Is it a simple GET request? → Use `http.fetch`
├─ Is it OpenAI/Anthropic? → Use `openai.chat_completion` / `anthropic.chat_completion`
├─ Is it a custom API they'll use multiple times?
│  ├─ Simple API calls? → Create JSON manifest
│  └─ Need caching/retries/business logic? → Create MCP server
├─ Is it a one-off request? → Use `http` with full control
├─ Database operation? → Use `mcp://postgres/` or similar
└─ File operation? → Use `mcp://filesystem/` or similar
```

### Recommended Approach for Custom APIs:

**✅ Good: Create a reusable manifest**
```yaml
# First, create .beemflow/registry.json with:
{
  "type": "tool", 
  "name": "shopify.products.list",
  "endpoint": "https://mystore.myshopify.com/admin/api/2023-01/products.json",
  "headers": {"X-Shopify-Access-Token": "$env:SHOPIFY_TOKEN"}
}

# Then use it simply:
- id: get_products
  use: shopify.products.list
  with:
    limit: 50
```

**❌ Avoid: Repeating HTTP config**
```yaml
# Don't repeat this across multiple flows:
- id: get_products
  use: http
  with:
    url: "https://mystore.myshopify.com/admin/api/2023-01/products.json"
    headers:
      X-Shopify-Access-Token: "{{ secrets.SHOPIFY_TOKEN }}"
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
      text: "Hello, {{ event.user }}!"
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
      <field>: <value>       # e.g. record_id: "{{ some_id }}", field: Status, equals: Approved
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
      record_id: "{{ outputs.create_airtable_record.id }}"
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
        request_id: "{{ event.request_id }}"
        status: approved
      timeout: 48h
  - id: notify
    use: core.echo
    with:
      text: "Approval received for request {{ event.request_id }}!"
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
- **Templating:** `{{ outputs.step.field }}`, `{{ vars.NAME }}`, helpers
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
      text: "{{ outputs.greet.text }}"
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
| List flows        | `flow list`                  | `GET /flows`                 | `beemflow_list_flows`       |
| Get flow          | `flow get <name>`            | `GET /flows/{name}`          | `beemflow_get_flow`         |
| Validate flow     | `flow validate <name_or_file>` | `POST /validate`             | `beemflow_validate_flow`    |
| Lint flow         | `flow lint <file>`           | `POST /flows/lint`           | `beemflow_lint_flow`        |
| Graph flow        | `flow graph <name_or_file>`  | `POST /flows/graph`          | `beemflow_graph_flow`       |
| Run flow          | `flow start <flow-name>`     | `POST /runs`                 | `beemflow_start_run`        |
| Get run status    | `flow get-run <run_id>`      | `GET /runs/{id}`             | `beemflow_get_run`          |
| List runs         | `flow list-runs`             | `GET /runs`                  | `beemflow_list_runs`        |
| Resume run        | `flow resume <token>`        | `POST /resume/{token}`       | `beemflow_resume_run`       |
| Publish event     | `flow publish <topic>`       | `POST /events`               | `beemflow_publish_event`    |
| **🛠️ Tool Manifests** |                           |                              |                            |
| Search tools      | `flow tools search [query]`  | `GET /tools/search`          | `beemflow_search_tools`     |
| Install tool      | `flow tools install <tool>`  | `POST /tools/install`        | `beemflow_install_tool`     |
| List tools        | `flow tools list`            | `GET /tools`                 | `beemflow_list_tools`       |
| Get tool manifest | `flow tools get <name>`      | `GET /tools/{name}`          | `beemflow_get_tool_manifest` |
| **🖥️ MCP Servers**   |                           |                              |                            |
| Search servers    | `flow mcp search [query]`    | `GET /mcp/search`            | `beemflow_search_mcp`      |
| Install server    | `flow mcp install <server>`  | `POST /mcp/install`          | `beemflow_install_mcp`     |
| List servers      | `flow mcp list`              | `GET /mcp`                   | `beemflow_list_mcp`        |
| Serve MCP         | `flow mcp serve`             | N/A                          | N/A                        |
| **⚙️ General**       |                           |                              |                            |
| Convert OpenAPI   | `flow convert <openapi_file>`| `POST /tools/convert`        | `beemflow_convert_openapi`  |
| Show spec         | `flow spec`                  | `GET /spec`                  | `beemflow_spec`             |
| Test flow         | `flow test`                  | `POST /flows/test`           | `beemflow_test_flow`        |

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
      record_id: "{{ airtable_row.id }}"
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
      token:   "{{ secrets.SLACK_TOKEN }}"
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
      text: "{{ outputs.greet.text }}"
```

### Fetch and Summarize

```yaml
name: fetch_and_summarize
on: cli.manual
steps:
  - id: fetch
    use: http.fetch
    with:
      url: "https://api.example.com/data"
  - id: summarize
    use: openai.chat_completion
    with:
      model: "gpt-4o"
      messages:
        - role: system
          content: "Summarize the following text in 3 bullet points."
        - role: user
          content: "{{ outputs.fetch.body }}"
  - id: print
    use: core.echo
    with:
      text: "{{ summarize.choices.0.message.content }}"
```

---

## 12. Extensibility Patterns

- **Add a local tool:** `flow tools install <registry>:<tool>` or add entries to `.beemflow/registry.json`.
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
