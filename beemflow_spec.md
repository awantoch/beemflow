
AGENTFLOW — ULTRA‑COMPREHENSIVE SPECIFICATION
==============================================

──────────────────────────────────────────────
1. PURPOSE & VISION
──────────────────────────────────────────────
• Text‑first, open protocol to define and run AI‑powered, event‑driven automations.  
• One `.flow.yaml` file expresses:
  – Triggers (webhook / cron / bus / CLI)  
  – Steps (tool calls)  
  – Control‑flow (if, loop, parallel)  
  – Retries & back‑offs  
  – Durable waits on external callbacks  
  – Error routing (`catch`)  
• Spec‑first, GUI‑later: file is Git‑diff‑able source‑of‑truth.  
• Runtime is plug‑and‑play: storage/blob/event back‑ends chosen via JSON.  
• Integrations are "tools" described by JSON‑Schema manifests (OpenAI tool spec).  
• Supports MCP endpoints (`mcp://server/tool`) + GitHub‑sourced adapters.  
• Flows run unchanged in SaaS cloud or self‑hosted engine.

──────────────────────────────────────────────
2. YAML FILE GRAMMAR
──────────────────────────────────────────────
```yaml
name:       string                       # required
version:    string                       # optional semver
on:         list|object                  # triggers
vars:       map[string]                  # optional constants / secret refs
steps:      array of step objects         # required
catch:      map[label] → step            # optional global error flow
```

**Trigger kinds**
```yaml
on:
  - event: webhook.shopify.order_created   # manifest auto‑registers webhook
  - cron:  "0 2 1 * *"                     # 02:00 on 1st monthly
  - eventbus.inventory.low_stock
  - cli.manual
```

──────────────────────────────────────────────
3. STEP DEFINITION KEYS
──────────────────────────────────────────────
```yaml
label:                                  # e.g., fetch_tweet:
  use: string                           # tool identifier
  with: object                          # args validated by manifest
  if:  expression                       # skip/branch
  foreach: expression                   # loop array
    as: string                          # loop var
    do: sequence                        # nested steps
  parallel: [ path1, path2, … ]         # fan‑out / fan‑in
  retry: { attempts: n, delay_sec: m }  # automatic retry
  await_event:                          # durable wait
    source: string
    match:  object
    timeout: 7d|5h|60s
  wait: { seconds: 30 } | { until: ts } # sleep
```

**Templating** `{{ … }}`  
Scopes: `event`, `vars`, previous step outputs (`label.field`), loop locals, helper funcs (`now()`, `duration(n,'days')`, `join`, `map`, `length`, `base64()`, etc.).

──────────────────────────────────────────────
4. TOOL IDENTIFIER RESOLUTION
──────────────────────────────────────────────
Priority order when resolving `use:` value:

1. **Local manifests**: `/tools/<name>.json`.
2. **Community hub**: `https://hub.beemflow.com/index.json` (supports `tool@version`).
3. **MCP servers**: `mcp://server/tool` → fetch manifest at `/.well-known/beemflow.json`.
4. **GitHub shorthand**: `github:owner/repo[/path][@ref]`
   • default `path=tools/<tool>.json`, default `ref=main`.  
   • Runtime clones / archives repo at `ref`, caches manifest.

──────────────────────────────────────────────
5. TOOL MANIFEST (JSON‑SCHEMA)
──────────────────────────────────────────────
```jsonc
{
  "name": "shippo.label.create",
  "description": "Buy a shipping label via Shippo",
  "kind": "task",               // or "event"
  "parameters": {
    "type": "object",
    "required": ["order_id","ship_from","ship_to"],
    "properties": {
      "order_id":  { "type": "string" },
      "ship_from": { "$ref": "#/definitions/address" },
      "ship_to":   { "$ref": "#/definitions/address" }
    },
    "definitions": {
      "address": {
        "type": "object",
        "required": ["name","street","city","zip"],
        "properties": {
          "name":   { "type": "string" },
          "street": { "type": "string" },
          "city":   { "type": "string" },
          "zip":    { "type": "string" }
        }
      }
    }
  },
  "event": {                    // present only if kind="event"
    "source": "shopify",
    "topic": "orders/create",
    "sample": { /* webhook example */ }
  }
}
```

──────────────────────────────────────────────
6. DURABLE WAITS & CALLBACKS
──────────────────────────────────────────────
`await_event` step → adapter returns `WAIT(token)` → runtime:
• Saves run/step state to Storage adapter.  
• Inserts token into `events_waiting`.  
External system calls `POST /resume/{token}` (HMAC‑signed) → runtime loads run, removes wait, continues DAG.

──────────────────────────────────────────────
7. RUNTIME HTTP API
──────────────────────────────────────────────
```
POST /runs              { flow_id, event }         → { run_id, status }
GET  /runs/{id}                                       status + outputs
POST /resume/{token}                                  resume paused run
GET  /files/{file_id}                                 presigned blob
```

──────────────────────────────────────────────
8. ADAPTER INTERFACES
──────────────────────────────────────────────
```ts
// Storage
interface Storage {
  saveRun(run: Run):               Promise<void>;
  getRun(id: UUID):                Promise<Run|null>;
  saveStep(step: Step):            Promise<void>;
  getSteps(runId: UUID):           Promise<Step[]>;
  registerWait(token: UUID, wakeAt?:Date): Promise<void>;
  resolveWait(token: UUID):        Promise<Run|null>;
}

// Blob Store
interface BlobStore {
  put(buf:ArrayBuffer, mime:string, filename?:string): Promise<string>; // URL
  get(url:string): Promise<ArrayBuffer>;
}

// Event Bus
interface EventBus {
  publish(topic:string, payload:any): Promise<void>;
  subscribe(topic:string, cb:(p:any)=>void): Unsub;
}
```
Reference adapters:  
Storage `memory | sqlite | postgres | dynamo | cockroachdb`  
Blob    `inline-base64 | s3 | gcs | minio`  
Event   `in-proc | redis | nats | sns`

──────────────────────────────────────────────
9. RUNTIME CONFIG (JSON)
──────────────────────────────────────────────
```json
{
  "storage": { "driver": "postgres", "dsn": "postgres://user:pw@host/db" },
  "blob":    { "driver": "s3",       "bucket": "beemflow-files" },
  "event":   { "driver": "redis",    "url": "redis://host:6379" },
  "registries": [
    "https://hub.beemflow.com/index.json",
    "https://raw.githubusercontent.com/my-org/tools/main/index.json"
  ],
  "mcp_servers": {
    "supabase-mcp.cursor.directory": {
      "install_cmd": ["npx", "supabase-mcp-server"],
      "required_env": ["SUPABASE_URL", "SUPABASE_SERVICE_ROLE_KEY"],
      "port": 3030
    }
  }
}
```
Omit adapters for dev quick‑start: falls back to in‑memory + base64.

──────────────────────────────────────────────
10. PROJECT SKELETON
──────────────────────────────────────────────
```
my-beemflow/
├ flows/                 # YAML specs
│  └ my_flow.flow.yaml
├ tools/                 # optional manifests
├ adapters/              # optional custom adapters
├ runtime.config.json    # selects back‑ends
├ .env                   # secrets (ignored)
└ README.md
```

──────────────────────────────────────────────
11. CLI COMMANDS
──────────────────────────────────────────────
```bash
flow serve --config runtime.config.json    # start engine
flow run [--config runtime.config.json] <flow> --event event.json         # run once (with optional config)
flow lint <file>                           # validate spec
flow graph <file> -o diagram.svg           # Mermaid DAG
flow tool scaffold <tool.name>             # generate manifest+stub
flow validate <file> [--dry-run]           # validate and simulate a flow without executing adapters
flow test <file>                           # run unit tests for a flow using mock adapters
```

──────────────────────────────────────────────
12. SECURITY PRACTICES
──────────────────────────────────────────────
• Secrets only in env/Vault → refer as `{{secrets.KEY}}`.  
• `await_event` callbacks: HMAC signature.  
• Step‑level timeout/memory caps.  
• SQL Row‑Level Security for multi‑tenant.  
• On error → dump `context` JSON via `blob.upload` for forensics.

──────────────────────────────────────────────
13. COMPLETE EXAMPLE FLOWS
──────────────────────────────────────────────

───────────────────────────────
A. HELLO WORLD (10 lines)
───────────────────────────────
```yaml
name: hello
on:   cli.manual
steps:
  - id: greet
    use: openai.chat
    with: { system: "Friendly AI", text: "Hello, world!" }
  - id: print
    use: core.echo
    with: { text: "{{greet.text}}" }
```

───────────────────────────────
B. TWITTER → INSTAGRAM (30 lines)
───────────────────────────────
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
    use: openai.chat
    with:
      text: "{{fetch_tweet.text}}"
      style: "instagram"

  - id: post_instagram
    use: instagram.media.create
    with:
      caption: "{{rewrite.text}}"
      image_url: "{{fetch_tweet.media_url}}"
```

───────────────────────────────
C. MARKETING "LAUNCH BLAST" MULTI‑CHANNEL
───────────────────────────────
```yaml
name: launch_blast
on:
  - webhook.product_feature

vars:
  wait_between_polls: 30

steps:
  search_docs:
    use: docs.search
    with: { query: "{{event.feature}}", top_k: 5 }

  marketing_context:
    use: openai.chat
    with:
      system:  "You are product marketing."
      text: |
        ### Feature
        {{event.feature}}
        ### Docs
        {{search_docs.results | join('\n\n')}}
      max_tokens: 400

  gen_copy:
    use: openai.chat
    with:
      function_schema: |
        { "name":"mk_copy","parameters":{
          "type":"object","properties":{
            "twitter":{"type":"array","items":{"type":"string"}},
            "instagram":{"type":"string"},
            "facebook":{"type":"string"}
        }}}
      prompt: |
        Write 3 Tweets, 1 IG caption, 1 FB post about:
        {{marketing_context.summary}}

  airtable_row:
    use: airtable.records.create
    with:
      base_id: "{{secrets.AIR_BASE}}"
      table:   "Launch Copy"
      fields:
        Feature:   "{{event.feature}}"
        Twitter:   "{{gen_copy.twitter | join('\n\n---\n\n')}}"
        Instagram: "{{gen_copy.instagram}}"
        Facebook:  "{{gen_copy.facebook}}"
        Status:    "Pending"

  await_approval:
    await_event:
      source: airtable
      match:
        record_id: "{{airtable_row.id}}"
        field:     Status
        equals:    Approved

  publish_all:
    parallel:
      - path: push_twitter
      - path: push_instagram
      - path: push_facebook

  push_twitter:
    foreach: "{{gen_copy.twitter}}"
    as: tweet
    do:
      - step_id: post_tw
        use: twitter.tweet.create
        with: { text: "{{tweet}}" }

  push_instagram:
    use: instagram.media.create
    with:
      caption: "{{gen_copy.instagram}}"
      image_url: "{{event.image_url}}"

  push_facebook:
    use: facebook.post.create
    with: { message: "{{gen_copy.facebook}}" }

  done:
    use: core.log.info
    with: { message: "Launch blast complete" }
```

───────────────────────────────
D. HVAC DISPATCH & INVOICE (60 lines)
───────────────────────────────
```yaml
name: hvac_dispatch_invoice
on:
  - event: airtable.records.create
    table: "Service Requests"

vars:
  invoice_terms: "Net 15"
  payment_grace_days: 5

steps:
  get_tech:
    use: techfinder.closest_oncall
    with:
      zipcode: "{{event.fields.Zip}}"
      skill:   "{{event.fields.IssueType}}"

  schedule_calendar:
    use: google.calendar.event.create
    with:
      calendar_id: "field-techs@hvac.com"
      title: "Service – {{event.fields.IssueType}}"
      start: "{{now()}}"
      end:   "{{now() + duration(2,'hours')}}"
      attendees:
        - "{{get_tech.email}}"
        - "{{event.fields.CustomerEmail}}"

  update_row:
    use: airtable.records.update
    with:
      base_id: "{{event.base_id}}"
      table:   "Service Requests"
      record_id: "{{event.record_id}}"
      fields:
        Tech: "{{get_tech.name}}"
        Status: "Scheduled"

  await_complete:
    await_event:
      source: airtable
      match:
        record_id: "{{event.record_id}}"
        field: Status
        equals: Completed
      timeout: 7d

  create_invoice:
    use: quickbooks.invoice.create
    with:
      customer_email: "{{event.fields.CustomerEmail}}"
      items:
        - description: "{{event.fields.IssueType}} repair"
          qty: 1
          rate: "{{event.fields.EstimatedCost}}"
      terms: "{{vars.invoice_terms}}"

  capture_payment:
    use: stripe.payment_intent.create
    with:
      customer_email: "{{event.fields.CustomerEmail}}"
      amount: "{{event.fields.EstimatedCost}}"
      invoice_id: "{{create_invoice.id}}"

  payment_failed:
    if: "{{capture_payment.status != 'succeeded'}}"

  wait_grace:
    if: payment_failed
    wait:
      until: "{{now() + duration(vars.payment_grace_days,'days')}}"

  reminder_text:
    if: payment_failed
    use: openai.chat
    with:
      system: "Friendly collections assistant."
      text: |
        Compose a gentle reminder for invoice {{create_invoice.number}}.

  send_reminder:
    if: payment_failed
    use: email.send
    with:
      to: "{{event.fields.CustomerEmail}}"
      subject: "Reminder: Invoice {{create_invoice.number}}"
      body: "{{reminder_text.text}}"

catch:
  alert_ops:
    use: slack.chat.postMessage
    with:
      channel: "#dispatch-alerts"
      text: "HVAC flow error {{error.message}}"
```

───────────────────────────────
E. BOOKKEEPING MONTH‑END CLOSE (FULL)
───────────────────────────────
```yaml
name: month_end_close
on:
  - cron: "0 2 1 * *"     # 2 AM on 1st of each month

vars:
  payment_terms: "Net 15"

steps:
  foreach_client:
    foreach: "{{secrets.CLIENT_LIST}}"   # env JSON array
    as: client
    do:
      - step_id: bank_txns
        use: plaid.transactions.get
        with:
          client_id: "{{client.id}}"
          start: "{{first_day_prev_month()}}"
          end:   "{{last_day_prev_month()}}"

      - step_id: categorize
        use: openai.chat
        with:
          function_schema: |
            { "name":"categorize","parameters":{
              "type":"object","properties":{
                "txns":{"type":"array","items":{"type":"object"}}
          }}}
          prompt: |
            Categorize the following for Xero:
            {{bank_txns.transactions}}

      - step_id: draft
        use: xero.journal.create_draft
        with:
          client_id: "{{client.id}}"
          entries: "{{categorize.txns}}"

      - step_id: email_client
        use: email.send
        with:
          to: "{{client.email}}"
          subject: "Month‑end draft ready"
          body: |
            Please reply "Approved" to post.
          attachments: []

      - step_id: await_ok
        await_event:
          source: email
          match:
            thread_id: "{{email_client.thread_id}}"
            subject_contains: "Approved"
          timeout: 14d

      - step_id: post
        use: xero.journal.post
        with:
          draft_id: "{{draft.id}}"

      - step_id: archive_pdf
        use: blob.upload
        with:
          data_base64: "{{post.pdf_base64}}"
          mime: "application/pdf"
          filename: "{{client.name}}-{{prev_month_string()}}.pdf"
```

───────────────────────────────
F. SAAS RELEASE NOTES PIPELINE (FULL)
───────────────────────────────
```yaml
name: release_notes
on:
  - event: github.push
    branch: main

steps:
  list_commits:
    use: github.api.list_commits
    with:
      range: "{{event.before}}..{{event.after}}"

  summarise:
    use: openai.chat
    with:
      system: "Rewrite commit messages to user‑friendly changelog."
      text: "{{list_commits.commits | map('message') | join('\n')}}"
      max_tokens: 300

  notion_page:
    use: notion.page.create
    with:
      database_id: "{{secrets.NOTION_CHANGELOG_DB}}"
      title: "Release {{event.after | short_sha}} — {{today()}}"
      content: "{{summarise.text}}"

  cms_post:
    use: github:my-org/cms-adapter@main/tools/cms.post.json
    with:
      slug:  "{{today() | date_slug}}"
      title: "Release Notes — {{today()}}"
      body:  "{{summarise.text}}"

  tweet:
    use: twitter.tweet.create
    with:
      text: "{{summarise.text | first_240_chars}} 🚀"

  email_draft:
    use: mailchimp.campaign.create_draft
    with:
      list_id: "{{secrets.MC_LIST}}"
      subject: "What's new — {{today()}}"
      html_body: "{{summarise.text | markdown_to_html}}"

done:
  use: core.log.info
  with: { message: "release_notes flow complete" }
```

──────────────────────────────────────────────
MCP CLIENT SUPPORT
──────────────────────────────────────────────

BeemFlow is a first-class MCP (Model Context Protocol) client. This means:

• BeemFlow can connect to any MCP server (Node.js, Python, Go, Java, etc.) using HTTP or stdio transports.
• At runtime, BeemFlow discovers available tools by calling the `tools/list` method on the MCP server. Each tool provides its name, description, and input schema (if available).
• To invoke a tool, BeemFlow sends a `tools/call` request with the tool name and arguments. The server executes the tool and returns the result.
• No static manifest is required for MCP tools. BeemFlow will use the schema provided by the server at runtime. If no schema is provided, users can supply arguments as raw JSON.
• This approach ensures maximum compatibility with the MCP ecosystem, including Node.js servers used by Cursor, Claude, and others.

Example MCP tool usage in a flow:
```yaml
steps:
  - query_supabase:
      use: mcp://supabase-mcp.cursor.directory/supabase.query
      with:
        sql: "SELECT * FROM users"
```

BeemFlow will:
1. Connect to the MCP server at `supabase-mcp.cursor.directory`.
2. Call `tools/list` to discover available tools and their schemas.
3. Call `tools/call` with the tool name and arguments.
4. Return the result as the step output.

──────────────────────────────────────────────
14. LICENSE
──────────────────────────────────────────────
Spec + reference runtime: **MIT**.  
Adapters default MIT unless otherwise noted.

──────────────────────────────────────────────
15. ROADMAP
──────────────────────────────────────────────
• VSCode extension (syntax highlight, lint, live Mermaid).  
• Optional web canvas (edits YAML).  
• Flow template gallery (`flow init`).  
• Temporal backend adapter.  
• Metrics / observability plugin.  

──────────────────────────────────────────────
END OF SPEC
──────────────────────────────────────────────
