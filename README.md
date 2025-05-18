# BeemFlow 🚀
### The Open Protocol for AI-Powered, Event-Driven Automations

[![Star](https://img.shields.io/github/stars/awantoch/beemflow?style=social)](https://github.com/awantoch/beemflow)

**TL;DR:** Write, share, and run AI-driven workflows in YAML. 100% open, Git-friendly, vendor-neutral.

---

## 🚀 Try It Now

1. **Clone the repo:**
   ```bash
   git clone https://github.com/awantoch/beemflow.git
   cd beemflow
   ```
2. **Run an example flow:**
   ```bash
   flow run hello
   # or add your OPENAI_API_KEY to .env and try another example:
   flow run fetch_and_summarize
   flow run parallel_openai
   ```

> **Note:** You only need to provide `--event` if your flow references fields from the event (e.g. `{{event.something}}`). For most CLI/manual flows, you can omit it.

---

## 🌟 Why Now?

AI is everywhere. APIs are everywhere. Yet automation is still stuck in siloed, proprietary platforms. It's time for a revolution.

Meet **BeemFlow** – *GitHub Actions* for AI workflows:

- Text-first: human-readable, versioned in Git
- Vendor-neutral: run on cloud, self-hosted, or embed anywhere
- Community-driven: remix, share, and build on top using interoperable standards
- Extensible: plug in LLMs, webhooks, cron, custom adapters
- Easy to use: chat + GUI visual workflow builder for technical and non-technical users

---

## 🔥 Key Highlights

1. **YAML-First**: Human-readable, Git-friendly, zero-lock-in
2. **AI-Native**: LLM chat, function calls, code executions
3. **Control Flow**: `if`, `foreach`, `parallel`, retries & back-offs
4. **Durable Waits**: Pause for external callbacks, resume seamlessly
5. **Pluggable Tools**: JSON-Schema–based manifests, local & hub discovery
6. **MCP Client**: **BeemFlow is a true MCP client**—it can connect to any MCP server (Node.js, Python, Go, etc.), dynamically discover tools at runtime via the `tools/list` method, and invoke them via `tools/call`, supporting both HTTP and stdio transports. Configure server installation, environment variables, and ports under the `mcp_servers` section of your runtime config for transparent auto-installation and startup. No static manifest is required for MCP tools; BeemFlow uses the schema provided by the server, or allows raw JSON if none is present.
7. **Any Backend**: Postgres, S3, Redis, SQLite, in-memory
8. **CLI, API, GUI**: Lint, run, serve, graph, scaffold—dev workflow optimized

---

## 🔍 Spec Primer

BeemFlow flows are defined in a single YAML file with a concise, expressive grammar:

Top-level keys:
- **name** (string, required)
- **version** (semver, optional)
- **on** (trigger list or object): supports `event`, `cron`, `eventbus`, `cli`
- **vars** (map): static constants or secret references
- **steps** (ordered list): each step is an object with an `id`
- **catch** (list): global error handlers

Step definition keys:
- `id`: Unique identifier for the step (required)
- `use`: tool identifier (JSON-Schema manifest)
- `with`: input arguments for the tool
- `if`: conditional expression to skip or branch
- `foreach`: loop over an array
  - `as`: loop variable
  - `do`: nested sequence of steps
- `parallel`: (optional) Boolean, if true, runs all nested `steps:` in parallel (nested/block parallel only)
- `retry`: `{ attempts: n, delay_sec: m }`
- `await_event`: durable wait on external callback
  - `source`, `match`, `timeout`
- `wait`: sleep for `{ seconds: n }` or `{ until: ts }`
- `depends_on`: (optional) List of step ids this step depends on

> **Note:** Only `parallel: true` with nested `steps:` is supported for parallel execution. This is to keep the flow simple and easy to interpret.

Templating & helpers:
- Interpolate values with `{{ … }}` (access `event`, `vars`, previous outputs)
- Built-in functions: `now()`, `duration(n,'unit')`, `join()`, `map()`, `length()`, `base64()`, etc.

Tool identifier resolution (in order):
1. Local manifests (`tools/<name>.json`)
2. Community hub indexes (e.g. https://hub.beemflow.com)
3. MCP servers (`mcp://server/tool`)
4. GitHub shorthand (`github:owner/repo[/path][@ref]`)

### MCP Tool Example (No Manifest Required)

You can use any MCP tool directly, even if no static manifest is present. BeemFlow will discover the tool and its schema at runtime:

```yaml
steps:
  - id: query_supabase
    use: mcp://supabase-mcp.cursor.directory/supabase.query
    with:
      sql: "SELECT * FROM users"
```

BeemFlow will:
1. Connect to the MCP server at `supabase-mcp.cursor.directory`.
2. Call `tools/list` to discover available tools and their schemas.
3. Call `tools/call` with the tool name and arguments.
4. Return the result as the step output.

### Nested Parallel Block Example (Preferred)

```yaml
steps:
  - id: fanout
    parallel: true
    steps:
      - id: chat1
        use: openai
        with:
          model: "gpt-3.5-turbo"
          messages:
            - role: user
              content: "Prompt 1"
      - id: chat2
        use: openai
        with:
          model: "gpt-3.5-turbo"
          messages:
            - role: user
              content: "Prompt 2"
  - id: combine
    depends_on: [fanout]
    use: core.echo
    with:
      text: |
        Combined responses:\n
        - chat1: {{.outputs.fanout.chat1.choices[0].message.content}}
        - chat2: {{.outputs.fanout.chat2.choices[0].message.content}}
```

The parent step (`fanout`) runs all its children in parallel and is considered complete when all are done. Downstream steps can depend on the parent for fan-in.

---

## 💡 25 High-Value Use Cases
- Automate Shopify order fulfillment: payment → shipping label → CRM update → customer notification
- Proactively prevent customer churn: daily risk scoring → personalized win-back emails → CRM flags
- Launch multi-channel marketing from one spec: Airtable → Twitter, Instagram, Facebook
- Publish GitHub release notes to Notion, CMS, Twitter & email
- Syndicate tweets to Instagram with AI-tailored captions
- Process e-commerce returns & refunds: webhook → Stripe refund → email & Slack alert
- Close books monthly: pull bank transactions → draft Xero journals → send reminders
- Dispatch field technicians: geo-match on-call tech → calendar invite → status update
- Onboard new users: create accounts → send welcome emails → Slack notifications
- Roll out feature flags: schedule toggles → monitor metrics → auto-roll back
- Auto-remediate incidents: detect errors → trigger fix script → report results
- Monitor social sentiment: analyze tweets → classify sentiment → alert teams
- Run NPS surveys: send surveys → collect responses → summarize insights
- Triage support tickets: classify requests → assign priority → notify teams
- Qualify leads: enrich via Clearbit → score leads → generate CRM tasks
- Adjust pricing in real time: analyze demand signals → update price → notify ops
- Automate billing & reminders: generate invoices → send emails → update records
- Generate compliance reports: extract data → format PDF → archive logs
- Streamline KYC flows: collect documents → verify identity → update user status
- Orchestrate data pipelines: ETL on schedule → transform → load into warehouse
- Automate HR onboarding: create accounts → assign permissions → send orientation materials
- Manage webinars: handle registrations → calendar invites → follow-up email series
- Monitor IoT devices: collect telemetry → detect anomalies → trigger alerts
- AI-powered code review: analyze PRs → post comments → notify authors
- Q&A chatbot for internal docs: fetch docs → answer Slack queries

## 💰 Cost Savings & ROI
- Replace 6–10 automation services (Zapier, Make, n8n, etc.) at $200/mo each with one BeemFlow stack: save $14K–$17K per department/year.
- Consolidating 10 custom connectors saves $150K–$300K in engineering costs.
- Eliminates per-tool security and maintenance overhead: ~$50K in annual ops savings.
- **Total First-Year Savings** for a 5-team org: $300K–$500K; ROI in under 3 months.

---

## 🛑 Consolidate & Orchestrate Core Services
- **Integration & Orchestration** (Zapier, Make, n8n, Workato): Consolidate multi-API workflows into version-controlled YAML flows with built-in retries, parallelism, and durable waits.
- **Data Ingestion & ETL** (Fivetran, Stitch, Matillion, Talend): Schedule SQL extracts, apply LLM-powered transforms, and load into your data warehouse—no more separate ETL pipelines or ETL tool subscriptions.
- **CRM & Marketing Workflows** (Salesforce Flow, HubSpot Workflows, Pardot): Automate lead routing, scoring, nurture sequences, and record updates as code, with full audit trails and no extra automation seats.
- **Billing & Invoicing** (Stripe Billing, QuickBooks, Xero): Create invoices, capture payments, reconcile records, and trigger follow-up notifications—all with one central flow.
- **Monitoring & Auto-Remediation** (Datadog, Prometheus, Grafana): Query metrics, detect anomalies, auto-scale or rollback services, and alert on-call engineers automatically.
- **Incident Response & Alerts** (PagerDuty, Opsgenie, VictorOps): Route alerts based on dynamic thresholds, invoke remediation scripts, and notify stakeholders via Slack or email.
- **Email Campaign Automation** (Mailchimp, SendGrid, Campaign Monitor): Generate, personalize, and schedule targeted email campaigns programmatically via your ESP API—no builders required.
- **Social Media Automation** (Hootsuite, Buffer, Sprout Social): Cross-post to multiple platforms with AI-generated captions and track engagement within a single flow.
- **Report Automation** (Metabase, Looker, Mode Analytics): Execute scheduled queries, format results into PDFs or dashboards, and distribute to stakeholders without manual intervention.
- **CI/CD Job Orchestration** (GitHub Actions, Jenkins, CircleCI): Trigger builds, monitor test outcomes, and notify teams—while specialized runners handle compilation and deployment.
- **Contract & Document Workflows** (DocuSign, PandaDoc, HelloSign): Generate agreements from JSON/Schema, send for signature, and track signing status in one cohesive pipeline.
- **Bot & Chat Automation** (Intercom, Drift, HubSpot Chatbot): Define event-driven chat sequences, leverage LLMs for context-aware responses, and escalate to human agents seamlessly.
- **CMS Content Sync** (Contentful, Strapi, Sanity, Ghost): Sync documentation or content updates, generate drafts, and publish changes—eliminating manual sync processes.
- **Form & Survey Processing** (Typeform, JotForm, SurveyMonkey): Ingest submissions, perform enrichment or sentiment analysis, and trigger customized follow-up workflows instantly.
- **Feedback & NPS Surveys** (Delighted, Wootric, Typeform NPS): Automate survey distribution, collect responses, analyze sentiment with AI, and summarize insights—no spreadsheets needed.

---

## 🛠️ Quickstart: Hello World

1. **Install** the CLI (coming soon)
2. **Create** `flows/hello.flow.yaml`:
   ```yaml
   name: hello
   on: cli.manual
   steps:
     - id: greet
       use: openai
       with:
         model: "gpt-4o"
         messages:
           - role: system
             content: "Please give a reply to the following message:"
           - role: user
             content: "Hello, BeemFlow!"
     - id: print
       use: core.echo
       with:
         text: "{{.outputs.greet.text}}"
   ```
3. **Run & Visualize**:
   ```bash
   flow serve --config flow.config.json
   flow run --config flow.config.json hello --event event.json
   flow graph flows/hello.flow.yaml -o hello.svg
   ```

## 🔐 Authentication & Secrets

BeemFlow uses a unified `secrets` scope to inject credentials, API keys, and HMAC keys into your flows securely. No special syntax—just load them into the runtime environment, event, or secrets backend and reference via `{{secrets.KEY}}`.

1. **Load your secrets**
   - Create a `.env` file or configure your runtime to read from Vault/AWS Secrets Manager:
     ```dotenv
     SLACK_TOKEN=xoxb-…
     GITHUB_TOKEN=ghp_…
     STRIPE_KEY=sk_…
     AWS_ACCESS_KEY_ID=AKIA…
     AWS_SECRET_ACCESS_KEY=…
     WEBHOOK_HMAC_KEY=supersecret
     OPENAI_API_KEY=your_openai_api_key
     ```

2. **Reference in your flow steps**
   ```yaml
   steps:
     - id: notify_ops
       use: slack.chat.postMessage
       with:
         channel: "#ops"
         text:    "All systems go!"
         token:   "{{secrets.SLACK_TOKEN}}"

     - id: create_pr
       use: github.api.create_pull_request
       with:
         repo:      "my-org/repo"
         title:     "Automated update"
         head:      "dep-update-2025-05-17"
         base:      "main"
         body:      "Dependency bump"
         token:     "{{secrets.GITHUB_TOKEN}}"
   ```

3. **Adapter defaults**
   Many adapter manifests declare default parameters from environment variables. If your Slack adapter sets `token: { "default": { "$env": "SLACK_TOKEN" } }`, you can omit `token:` entirely in the flow.
   Similarly, the OpenAI adapter (`openai`) sets `api_key` default from the `OPENAI_API_KEY` environment variable, so you can omit `api_key:` entirely when using `openai`.

4. **Secrets via event**
   Secrets can also be injected via the event payload, not just environment or secrets backend.

5. **Shell steps**
   Shell commands inherit the same environment:
   ```yaml
   - id: deploy
     use: shell.exec
     with:
       command: |
         aws s3 cp build/ s3://my-bucket/ --recursive
   ```
   Credentials like `AWS_ACCESS_KEY_ID` will be picked up automatically.

6. **Durable wait callbacks**
   For `await_event`, configure your HTTP adapter to verify HMAC signatures using `WEBHOOK_HMAC_KEY` from `secrets`, ensuring only valid resume requests succeed.

7. **AWS Secrets Manager**
   Instead of loading from `.env`, you can configure AWS Secrets Manager as a secrets backend:
   ```json
   {
     "secrets": {
       "driver": "aws-secrets-manager",
       "region": "us-east-1",
       "prefix": "/beemflow/"
     }
   }
   ```
   - Ensure the runtime host has AWS credentials (IAM role, or `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` and `AWS_REGION` env vars).
   - Secrets are looked up by name under the given prefix, e.g. `{{secrets.DB_PASSWORD}}` fetches the secret at `/beemflow/DB_PASSWORD`.

## ❓ FAQ

### Why does BeemFlow include a `tools/http.fetch.json` manifest but not one for `core.echo`?
BeemFlow treats `http.fetch` as a first-class, overridable JSON-Schema tool—we ship a manifest so teams can tweak its parameters (timeouts, headers, defaults) or swap in their own fetch implementation. In contrast, `core.echo` is an internal debug adapter with a fixed single `text` parameter and no need for runtime schema changes, so it has no external JSON manifest.

### Why is there an `openai.json` manifest if there's a built-in OpenAI adapter?
The Go adapter implements the execution logic (auth, HTTP calls, streaming, function-calling), but the JSON manifest declares the full input schema (all parameters, defaults, descriptions) so flows can validate and introspect the exact arguments—and so you can customize the schema without rebuilding the engine.

## 💼 Runtime Configuration

BeemFlow is driven by a JSON configuration file (default `flow.config.json`). You can also pass a custom path via `flow serve --config path/to/config.json`. Key sections:

```json
{
  "storage": { "driver": "postgres", "dsn": "postgres://user:pw@host/db" },
  "blob":    { "driver": "filesystem", "directory": "./beemflow-files" },
  "event":   { "driver": "redis",    "url": "redis://host:6379" },
  "secrets": { "driver": "aws-secrets-manager", "region": "us-east-1", "prefix": "/beemflow/" },
  "registries": [
    "https://hub.beemflow.com/index.json",
    "https://raw.githubusercontent.com/my-org/tools/main/index.json"
  ],
  "http": { "host": "0.0.0.0", "port": 8080 },
  "log":  { "level": "info" },
  "mcp_servers": {
    "airtable-mcp-server": {
      "install_cmd": ["npx", "-y", "airtable-mcp-server"],
      "required_env": ["AIRTABLE_API_KEY"],
      "port": 3030
    },
    "supabase-mcp-postgrest": {
      "install_cmd": ["npx", "-y", "@supabase/mcp-server-postgrest@latest"],
      "required_env": ["SUPABASE_URL", "SUPABASE_ANON_KEY"],
      "port": 3030
    }
  }
}
```

- **storage**: choose from `memory` (dev), `sqlite`, `postgres`, `dynamo`, `cockroachdb`.
- **blob**: choose from `filesystem` (default, recommended for local/dev/prod), `inline-base64` (dev), `s3`, `gcs`, `minio`.
- **event**: choose from `in-proc` (local), `redis`, `nats`, `sns`.
- **secrets**: configure how `{{secrets.KEY}}` is resolved (supported drivers: `env`, `aws-secrets-manager`, `vault`).
- **registries**: list of manifest index URLs for discovering community tools.
- **http**: server binding for the runtime's HTTP API (`/runs`, `/resume`, `/graph`, etc.).
- **log**: set log level (`debug`, `info`, `warn`, `error`).
- **mcp_servers**: map of MCP server addresses to their install commands, required environment variables, and optional ports.

Omit any section to use sensible defaults (in-memory adapters, built-in hubs, console logging). For development, you can skip `flow.config.json` entirely and BeemFlow will fall back to in-memory storage, inline blob encoding, an in-process event bus, and no registries.

### Local Filesystem Blob Store

The local filesystem blob store is the default and recommended option for most deployments. It stores blobs as files in a configurable directory on disk. Example configuration:

```json
{
  "blob": {
    "driver": "filesystem",
    "directory": "./beemflow-files"
  }
}
```

- `driver`: Must be set to `filesystem` to use the local disk blob store.
- `directory`: Path to the directory where blobs will be stored. This directory will be created if it does not exist.

### S3 and Other Cloud Blob Stores

For distributed or cloud-native deployments, you can use S3 or other cloud blob stores by changing the `driver` and providing the necessary configuration. Example:

```json
{
  "blob": {
    "driver": "s3",
    "bucket": "beemflow-files"
  }
}
```

See the documentation for more details on configuring S3, GCS, or Minio.

## 🛠 Official MCP Server Configurations

We've curated a set of official MCP server configurations in the `mcp_servers/` directory. These include:
- `airtable.json`
- `supabase.json`

To use one of these, simply add the desired server key (matching the filename in mcp_servers/) to the `mcp_servers` section of your `flow.config.json`:

```json
{
  "mcp_servers": {
    "airtable": {},
    "supabase": {}
  }
}
```

BeemFlow will automatically load the full configuration from `mcp_servers/<key>.json` for each key you specify. No need to copy or duplicate JSON objects.

## 🧩 Integration Guide

### Curating an MCP Server
Use this when you have an existing MCP-compatible process (Node.js, Python, Go, etc.) and want BeemFlow to auto-install, start, and invoke it.

• **Define a mapping**  
  Create `mcp_servers/<your-server>.json`:

```json
{
  "my-custom-mcp": {
    "install_cmd": ["npx","-y","my-mcp-server@latest"],
    "required_env": ["MY_MCP_KEY"],
    "port": 4000
  }
}
```

• **Merge into your runtime config**  
  Copy the JSON object under `"mcp_servers"` in `flow.config.json` (or `runtime.config.json`).
  BeemFlow will automatically merge the main config and any curated config from `mcp_servers/<host>.json` for each MCP server.

• **Set environment variables**  
  Ensure any `required_env` keys (e.g. `MY_MCP_KEY`) are in your shell or a `.env`.

• **Invoke in your flow**  
  No local manifest needed—BeemFlow will call `tools/list` at runtime:

```yaml
steps:
  - id: call_tool
    use: mcp://my-custom-mcp/toolName
    with:
      foo: "bar"
```

---

### Adding a Local HTTP‐Based Tool
Use this when you want a static, JSON-Schema–driven adapter against an HTTP API endpoint.

• **Drop a manifest**  
  Place `tools/<toolName>.json`:

```jsonc
{
  "name": "awesome.action",
  "description": "Does something awesome",
  "kind": "task",
  "parameters": {
    "type":"object",
    "required":["input"],
    "properties":{
      "input":{ "type":"string" }
    }
  },
  "endpoint": "https://api.awesome.com/do"
}
```

• **Auto-registration**  
  On startup, BeemFlow scans `tools/*.json` and registers every manifest as an HTTPAdapter. All tools in `tools/` are auto-registered.

• **Use in a flow**  
  Simply reference its `name`:

```yaml
steps:
  - id: epic
    use: awesome.action
    with:
      input: "data"
```

---

### When You Need a Custom Adapter
Most integrations fit into "MCP" or "HTTP-based tool" patterns. Only reach for a custom Go adapter if you must support:

- A new transport (e.g. gRPC, custom stdio protocol)  
- Complex input/output transformations beyond JSON-schema  

To build one:

• **Implement** the `Adapter` interface in Go (`ID()`, `Execute()`, `Manifest()`).  
• **Register** it in `engine/engine.go` before the auto-load of `tools/`.  
• **Invoke** by its adapter ID in your YAML:

```yaml
- id: custom_step
  use: myadapter
  with: { /* … */ }
```

---

### Best Practices
• **Filename matches key**  
  Name `mcp_servers/stripe.json` → key `"stripe"`.

• **Version control your JSON**  
  Keep all snippets under `mcp_servers/` or `tools/` in Git.

• **Document required env vars**  
  List them in your README or example flow.

• **Provide a tiny example**  
  Add a sample in `flows/`, e.g. `flows/my-custom-mcp.flow.yaml`.

• **Smoke-test**  
  Write a quick unit or integration test (mock HTTP, or verify `tools/list` succeeds).

---

With these patterns:  
- **MCP servers** need only a JSON mapping + env vars.  
- **HTTP tools** need only a manifest in `tools/`.  
- **Custom adapters** are a fallback for advanced cases.  

That's it—no extra wiring or code tweaks for the vast majority of integrations.

## 🖥️ CLI Commands

flow serve --config flow.config.json    # start the BeemFlow runtime
flow run [--config flow.config.json] <flow> --event <event.json>    # execute a flow once (with optional config)
flow lint <file>                        # validate your .flow.yaml against the spec
flow graph <flow> -o <diagram.svg>      # visualize your flow as a DAG
flow tool scaffold <tool.name>          # generate a tool manifest + stub
flow validate <file> [--dry-run]        # validate and simulate a flow without executing adapters
flow test <file>                        # run unit tests for a flow using mock adapters

---

## 🧪 Featured Example Flows

Below are real-world workflows to inspire your own automations.

### 1. Twitter → Instagram

Sync tweets to Instagram posts as images arrive:
```yaml
name: tweet_to_instagram
on:
  - event: webhook.twitter.tweet

steps:
  - id: fetch_tweet
    use: twitter.tweet.get
    with:
      id: "{{event.id}}"

  - id: rewrite
    use: openai
    with:
      model: "gpt-3.5-turbo"
      messages:
        - role: system
          content: "Rewrite the following text in an Instagram style: {{fetch_tweet.text}}"

  - id: post_instagram
    use: instagram.media.create
    with:
      caption: "{{ (index .rewrite.choices 0).message.content }}"
      image_url: "{{fetch_tweet.media_url}}"
```

---

### 2. Multi-Channel Marketing Blast

Automatically generate and publish marketing copy across Airtable, Twitter, Instagram, and Facebook:
```yaml
name: launch_blast
on:
  - event: webhook.product_feature

vars:
  wait_between_polls: 30

steps:
  - id: search_docs
    use: docs.search
    with:
      query: "{{.event.feature}}"
      top_k: 5

  - id: marketing_context
    use: openai
    with:
      model: "gpt-3.5-turbo"
      api_key: "{{secrets.OPENAI_API_KEY}}"
      system: "You are product marketing."
      text: |
        ### Feature
        {{.event.feature}}
        ### Docs
        {{.search_docs.results | join("\n\n")}}
      max_tokens: 400

  - id: gen_copy
    use: openai
    with:
      model: "gpt-3.5-turbo"
      api_key: "{{secrets.OPENAI_API_KEY}}"
      function_schema: |
        { "name": "mk_copy", "parameters": {
          "type": "object", "properties": {
            "twitter": {"type": "array", "items": {"type": "string"}},
            "instagram": {"type": "string"},
            "facebook": {"type": "string"}
        }}}
      prompt: |
        Write 3 Tweets, 1 IG caption, and 1 FB post about:
        {{.marketing_context.summary}}

  - id: airtable_row
    use: airtable.records.create
    with:
      base_id: "{{.secrets.AIR_BASE}}"
      table: "Launch Copy"
      fields:
        Feature: "{{.event.feature}}"
        Twitter: "{{.gen_copy.twitter | join("\n\n---\n\n")}}"
        Instagram: "{{.gen_copy.instagram}}"
        Facebook: "{{.gen_copy.facebook}}"
        Status: "Pending"

  - id: await_approval
    await_event:
      source: airtable
      match:
        record_id: "{{.airtable_row.id}}"
        field: Status
        equals: Approved

  - id: parallel
    - path: push_twitter
    - path: push_instagram
    - path: push_facebook

  - id: push_twitter
    foreach: "{{.gen_copy.twitter}}"
    as: tweet
    do:
      - id: post_tw
        use: twitter.tweet.create
        with:
          text: "{{.tweet}}"

  - id: push_instagram
    use: instagram.media.create
    with:
      caption: "{{.gen_copy.instagram}}"
      image_url: "{{.event.image_url}}"

  - id: push_facebook
    use: facebook.post.create
    with:
      message: "{{.gen_copy.facebook}}"
```

---

### 3. SaaS Release Notes Pipeline

Generate release notes on GitHub push, publish to Notion, CMS, and tweet:
```yaml
name: release_notes
on:
  - event: github.push
    branch: main

steps:
  - id: list_commits
    use: github.api.list_commits
    with:
      range: "{{event.before}}..{{event.after}}"

  - id: summarise
    use: openai
    with:
      model: "gpt-3.5-turbo"
      api_key: "{{secrets.OPENAI_API_KEY}}"
      system: "Rewrite commit messages into a user-friendly changelog."
      text: "{{list_commits.commits | map('message') | join('\n')}}"

  - id: notion_page
    use: notion.page.create
    with:
      database_id: "{{secrets.NOTION_CHANGELOG_DB}}"
      title: "Release {{event.after | short_sha}} — {{today()}}"
      content: "{{summarise.text}}"

  - id: cms_post
    use: github:my-org/cms-adapter@main/tools/cms.post.json
    with:
      slug: "{{today() | date_slug}}"
      title: "Release Notes — {{today()}}"
      body: "{{summarise.text}}"

  - id: tweet
    use: twitter.tweet.create
    with:
      text: "{{summarise.text | first_240_chars}} 🚀"

  - id: email_draft
    use: mailchimp.campaign.create_draft
    with:
      list_id: "{{secrets.MC_LIST}}"
      subject: "What's new — {{today()}}"
      html_body: "{{summarise.text | markdown_to_html}}"
```

---

### 4. E-Commerce Order Processing & Fulfillment

Automate the entire order-to-shipment lifecycle for your e-commerce store:

```yaml
name: ecommerce_order_processing
on:
  - event: webhook.shopify.order_created

vars:
  warehouse_name: "Acme Warehouse"
  warehouse_address:
    street: "123 Commerce St"
    city:   "Metropolis"
    zip:    "12345"

steps:
  - id: await_payment
    await_event:
      source: stripe
      match:
        payment_intent_id: "{{event.payment_intent_id}}"
        status: succeeded
      timeout: 1h

  - id: generate_label
    use: shippo.label.create
    with:
      order_id: "{{event.id}}"
      ship_from:
        name:   "{{vars.warehouse_name}}"
        street: "{{vars.warehouse_address.street}}"
        city:   "{{vars.warehouse_address.city}}"
        zip:    "{{vars.warehouse_address.zip}}"
      ship_to:
        name:   "{{event.shipping_address.name}}"
        street: "{{event.shipping_address.street}}"
        city:   "{{event.shipping_address.city}}"
        zip:    "{{event.shipping_address.zip}}"

  - id: create_fulfillment
    use: shopify.fulfillment.create
    with:
      order_id:        "{{event.id}}"
      tracking_number: "{{generate_label.tracking_number}}"
      notify_customer: true

  - id: update_crm_contact
    use: hubspot.contact.upsert
    with:
      email: "{{event.customer_email}}"
      properties:
        first_name: "{{event.shipping_address.name}}"
        order_id:   "{{event.id}}"
        tracking:   "{{generate_label.tracking_number}}"

  - id: update_crm_deal
    use: hubspot.deal.create
    with:
      properties:
        dealname:   "Order #{{event.id}}"
        amount:     "{{event.total_price}}"
        pipeline:   "ecommerce"
        dealstage:  "fulfilled"

  - id: send_email
    use: email.send
    with:
      to:      "{{event.customer_email}}"
      subject: "Your order #{{event.id}} is on its way!"
      body: |
        Hi {{event.shipping_address.name}},

        Your order #{{event.id}} has been shipped!
        Tracking: {{generate_label.tracking_number}}
        Label URL: {{generate_label.label_url}}

        Thanks for shopping with us.

  - id: log_success
    use: core.log.info
    with:
      message: "Order {{event.id}} processed and shipped: {{generate_label.tracking_number}}"

catch:
  - id: notify_ops
    use: slack.chat.postMessage
    with:
      channel: "#ecommerce-ops"
      text:    "Error processing order {{event.id}}: {{error.message}}"
```

---

### 5. AI-Driven Customer Churn Prevention

Proactively identify high-risk users and automatically send personalized win-back campaigns:

```yaml
name: churn_prevention
on:
  - cron: "0 8 * * *"   # Every day at 08:00 UTC

vars:
  crm_table: "Customers"
  churn_threshold: 0.7

steps:
  - id: fetch_usage
    use: analytics.query
    with:
      sql: |
        SELECT user_id, name, email, last_login, purchase_history
        FROM user_metrics

  - id: predict_churn
    use: openai
    with:
      function_schema: |
        { "name": "predict_churn", "parameters": { "type": "object", "properties": { "users": { "type": "array", "items": { "type": "object", "properties": { "user_id": {"type":"string"}, "name": {"type":"string"}, "email": {"type":"string"}, "last_login": {"type":"string"}, "purchase_history": {"type":"array","items":{"type":"object"}} } } } } } }
      prompt: |
        Given the following user data, predict a churn risk score (0.0–1.0) for each:
        {{.fetch_usage.results}}

  - id: retain
    foreach: "{{.predict_churn.churn_predictions}}"
    as: prediction
    do:
      - id: maybe_retain
        if: "{{.prediction.risk >= .vars.churn_threshold}}"
        do:
          - id: gen_offer
            use: openai
            with:
              system: "Retention Specialist"
              text: |
                Compose a personalized 20% discount win-back email for {{.prediction.name}} ({{.prediction.email}}).
          - id: send_email
            use: email.send
            with:
              to: "{{.prediction.email}}"
              subject: "We miss you, {{.prediction.name}}!"
              body: "{{.gen_offer.text}}"

catch:
  - id: notify_ops_churn
    use: slack.chat.postMessage
    with:
      channel: "#churn-alerts"
      text: "Churn prevention pipeline failed: {{.error.message}}"
```

---

### 6. Developer SaaS Marketing Agent

Plug your developer docs into a CMO-grade agent that generates marketing strategy, website copy, social posts, design briefs, and creates GitHub issues + Slack alerts:

```yaml
name: marketing_agent
on:
  - cli.manual

vars:
  product_name: "MySaaS"
  docs_url:     "https://docs.mysaas.com"
  github_repo:  "my-org/mysaas"

steps:
  - id: fetch_docs
    use: docs.search
    with:
      query: "{{vars.product_name}} developer documentation"
      top_k: 50

  - id: marketing_strategy
    use: openai
    with:
      system: "You are a CMO-level marketing strategist."
      text: |
        Analyze the following developer docs and propose a high-impact marketing plan for {{vars.product_name}}:
        {{.fetch_docs.results | join("\n\n")}}

  - id: website_copy
    use: openai
    with:
      system: "You are a UX copywriter."
      text: |
        Based on the marketing plan, write hero section copy, feature bullet points, and a memorable tagline for {{vars.product_name}}.

  - id: twitter_posts
    use: openai
    with:
      function_schema: |
        { "name": "mk_social", "parameters": {
          "type": "object", "properties": {
            "twitter": {"type":"array","items":{"type":"string"}},
            "linkedin": {"type":"string"}
          }
        }}
      prompt: |
        Generate 5 tweet threads and 1 LinkedIn post based on this marketing plan:
        {{.marketing_strategy.text}}

  - id: design_brief
    use: openai
    with:
      system: "You are a UI/UX design expert."
      text: |
        Create a design brief for a Figma mockup of the homepage hero section, including color palette, style, and imagery recommendations to match the copy:
        {{.website_copy.text}}

  - id: create_website_issue
    use: github.api.create_issue
    with:
      repo:  "{{vars.github_repo}}"
      title: "Marketing: Update homepage copy for {{vars.product_name}}"
      body: |
        **Hero & Features**
        {{.website_copy.text}}

        **Design Brief**
        {{.design_brief.text}}

  - id: create_social_issue
    use: github.api.create_issue
    with:
      repo:  "{{vars.github_repo}}"
      title: "Marketing: Schedule social media content"
      body: |
        **Twitter Threads**
        {{.twitter_posts.twitter | join("\n\n")}}

        **LinkedIn Post**
        {{.twitter_posts.linkedin}}

  - id: notify_team
    use: slack.chat.postMessage
    with:
      channel: "#marketing"
      text: |
        Marketing assets ready for *{{vars.product_name}}*:
        • Homepage issue: {{.create_website_issue.html_url}}
        • Social issue:   {{.create_social_issue.html_url}}

catch:
  - id: notify_ops_marketing
    use: slack.chat.postMessage
    with:
      channel: "#marketing"
      text: "Marketing agent failed: {{.error.message}}"
```

### 7. Automated Dependency Updater (Dependabot Replacement)

Automatically bump, commit, and PR your repo's dependencies with an AI-generated changelog:

```yaml
name: dependency_updater
on:
  - cron: "0 5 * * *"    # daily at 05:00 UTC

vars:
  repo_url:  "https://github.com/awantoch/your-repo.git"
  workdir:   "/tmp/repo"
  branch:    "dep-update-{{today()}}"

steps:
  - id: checkout
    use: shell.exec
    with:
      command: git clone {{vars.repo_url}} {{vars.workdir}}

  - id: bump_deps
    use: shell.exec
    with:
      command: |
        cd {{vars.workdir}}
        npx npm-check-updates -u
        npm install

  - id: show_diff
    use: shell.exec
    with:
      command: |
        cd {{vars.workdir}}
        git diff
    # captured as show_diff.stdout

  - id: commit_and_push
    use: shell.exec
    with:
      command: |
        cd {{vars.workdir}}
        git checkout -b {{vars.branch}}
        git add package.json package-lock.json
        git commit -m "chore(deps): bump to latest versions"
        git push origin {{vars.branch}}

  - id: create_pr
    use: github.api.create_pull_request
    with:
      repo: "awantoch/your-repo"
      title: "Automated dependency update — {{today()}}"
      head: "{{vars.branch}}"
      base: "main"
      body: "Updating dependencies to the latest versions."

  - id: pr_description
    use: openai
    with:
      model: "gpt-3.5-turbo"
      api_key: "{{secrets.OPENAI_API_KEY}}"
      system: "Release Note Assistant"
      text: |
        Here's the diff of the update:
        {{.show_diff.stdout}}

  - id: update_pr
    use: github.api.update_pull_request
    with:
      repo: "awantoch/your-repo"
      pr_number: "{{.create_pr.number}}"
      body: "{{.pr_description.text}}"

catch:
  - id: notify_ops_depbot
    use: slack.chat.postMessage
    with:
      channel: "#devops"
      text: "Dependency updater failed: {{.error.message}}"
```

*(Full spec & more examples in `beemflow_ultra_spec.txt`)*

---

## 🗂️ Project Layout

```
my-beemflow/
├── flows/                 # .flow.yaml files
├── tools/                 # JSON-Schema tool manifests
├── adapters/              # custom adapter implementations
├── flow.config.json    # backend & registry settings
└── README.md              # 👈 You're here
```

---

## 🤝 Join the Movement

BeemFlow is 100% **open**. We need YOU:

- Shape the spec
- Build adapters & UIs
- Share and remix flows
- Launch a SaaS or plugin on top

🌐 GitHub: https://github.com/awantoch/beemflow  
💬 Discord: https://discord.gg/your-invite  
📚 Docs: https://beemflow.com/docs

---

## 📜 License

MIT. Use it, remix it, ship it.

---

**BeemFlow: Power the AI automation revolution.**

## Flow Definition

Flows are defined in YAML. Steps are now defined as a list, not a map. Each step must have an `id` and can specify dependencies and parallel execution.

### Step Fields
- `id`: Unique identifier for the step (required)
- `use`: The tool or adapter to use
- `with`: Input parameters
- `depends_on`: (optional) List of step ids this step depends on
- `parallel`: (optional) Boolean, if true and no dependencies, can run concurrently; or an array of step IDs for block-parallel barrier (fan-in) support
- `if`: (optional) Templated boolean condition; step is skipped unless it renders to `true`

### Example

```yaml
steps:
  - id: fetch_page
    use: http.fetch
    with:
      url: "{{.vars.fetch_url}}"
  - id: fetch_meta
    use: http.fetch
    with:
      url: "{{.vars.meta_url}}"
    parallel: true
  - id: echo_html
    use: core.echo
    with:
      text: "{{.outputs.fetch_page.body}}"
    depends_on: [fetch_page]
  - id: summarize
    use: openai
    with:
      model: "o4-mini"
      api_key: "{{.secrets.OPENAI_API_KEY}}"
      messages:
        - role: user
          content: "{{.outputs.fetch_page.body}}"
    depends_on: [fetch_page]
  - id: print
    use: core.echo
    with:
      text: "Summary: {{ (index .outputs.summarize.choices 0).message.content }}"
    depends_on: [summarize]
```

### Output Referencing

Reference outputs from previous steps using their `id`:
- `outputs.fetch_page.body`
- `outputs.summarize.choices`
- For nested/parallel steps, use `.outputs.<parent_step>.<child_step>.<field>` (e.g., `.outputs.fanout.chat1.choices[0].message.content`).

### Execution Model
- Steps are executed in dependency order.
- Steps with no dependencies and `parallel: true` can run concurrently.
- Steps with dependencies wait for their dependencies to finish. 
- Only block-parallel (`parallel: true` with `steps:`) is supported; array form is not implemented (roadmap).

---

## 🛠️ Roadmap, Stubs, and Extensibility

BeemFlow is designed for extensibility and practical iteration. Some features are intentionally stubbed or in-memory only, with clear extension points:

- **Adapters:** Easy to add new tool adapters. See `engine/engine.go` for registration. All adapters must implement `ID()`, `Execute()`, and `Manifest()`. Optional `Close()`.
- **Registry loader/community hub:** Registry loader exists for community hub support, but deep integration is a roadmap feature.
- **Cron triggers, step-level resource limits, advanced event bus drivers, and adapter hot-reload are not yet implemented and are considered roadmap features.**

## MCP Server Config Compatibility

This project supports the community/Claude-style MCP server config format. Use this style in your config files or curated configs:

### Claude-style (camelCase)
```json
{
  "mcpServers": {
    "airtable": {
      "command": "npx",
      "args": ["-y", "airtable-mcp-server"],
      "env": { "AIRTABLE_API_KEY": "pat123.abc123" }
    }
  }
}
```

## 🏃‍♂️ Running Example Flows

BeemFlow comes with a set of example flows you can run out of the box. Below are some of the most useful to get started:

### Hello World

`flows/hello.flow.yaml`:
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
Run it:
```bash
flow run hello --event event.json
```

### Fetch and Summarize

`flows/fetch_and_summarize.flow.yaml`:
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
Run it:
```bash
flow run fetch_and_summarize --event event.json
```

### Parallel OpenAI (Fanout/Fanin)

`flows/parallel_openai.flow.yaml`:
```yaml
name: parallel_openai
on: cli.manual
steps:
  - id: fanout
    parallel: true
    steps:
      - id: chat1
        use: openai
        with:
          model: "gpt-3.5-turbo"
          messages:
            - role: user
              content: "Prompt 1"
      - id: chat2
        use: openai
        with:
          model: "gpt-3.5-turbo"
          messages:
            - role: user
              content: "Prompt 2"
  - id: combine
    depends_on: [fanout]
    use: core.echo
    with:
      text: |
        Combined responses:\n
        - chat1: {{.outputs.fanout.chat1.choices[0].message.content}}
        - chat2: {{.outputs.fanout.chat2.choices[0].message.content}}
```
Run it:
```bash
flow run parallel_openai --event event.json
```

---

## ⏸️ Durable Waits, Pause & Resume

BeemFlow supports **durable waits** using the `await_event` step. This allows a flow to pause execution and wait for an external event or callback (such as a webhook or approval) before resuming.

- When a step with `await_event` is reached, the flow is paused and its state is saved.
- The system waits for a matching external event (e.g., an HTTP callback, webhook, or manual approval) that satisfies the `match` criteria.
- Once the event is received, the flow resumes from where it left off.
- This enables long-running, event-driven workflows that can span minutes, hours, or even days.

**Example:**
```yaml
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

You can resume a paused flow by sending the appropriate event (e.g., via webhook or CLI/API call) that matches the `await_event` criteria.

---