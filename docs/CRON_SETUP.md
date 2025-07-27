# BeemFlow Cron Setup Guide

BeemFlow supports scheduled workflow execution through integration with external cron systems. This approach is simple, reliable, and leverages battle-tested scheduling infrastructure.

## How It Works

1. Define workflows with `on: schedule.cron` trigger
2. BeemFlow provides HTTP endpoints that trigger these workflows
3. Configure your cron system to call these endpoints

## Workflow Configuration

Add cron trigger to your workflow:

```yaml
name: daily_report
on: schedule.cron
cron: "0 9 * * *"  # 9 AM daily (for documentation)

steps:
  - id: generate_report
    use: my_tool
    with:
      type: daily
```

**Note:** The `cron` field is currently for documentation. The actual schedule is controlled by your cron system.

## Endpoints

### Global Endpoint
`POST /cron` - Triggers ALL workflows with `schedule.cron`

### Per-Workflow Endpoint  
`POST /cron/{workflow-name}` - Triggers a specific workflow

## Setup Options

### 1. Vercel (Serverless)

Add to `vercel.json`:
```json
{
  "crons": [{
    "path": "/cron",
    "schedule": "*/5 * * * *"
  }]
}
```

For security, set the `CRON_SECRET` environment variable in Vercel. BeemFlow will automatically verify this secret on incoming cron requests.

### 2. System Cron (Linux/Mac)

Add to crontab:
```bash
# Run all scheduled workflows every 5 minutes
*/5 * * * * curl -X POST http://localhost:3333/cron

# Or run specific workflows at their intended times
0 9 * * * curl -X POST http://localhost:3333/cron/daily_report
0 * * * * curl -X POST http://localhost:3333/cron/hourly_sync
```

### 3. Kubernetes CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: beemflow-scheduler
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: cron-trigger
            image: curlimages/curl
            args:
            - /bin/sh
            - -c
            - curl -X POST http://beemflow-service:3333/cron
          restartPolicy: OnFailure
```

### 4. GitHub Actions

```yaml
name: Trigger BeemFlow Workflows
on:
  schedule:
    - cron: '*/5 * * * *'
jobs:
  trigger:
    runs-on: ubuntu-latest
    steps:
      - name: Trigger workflows
        run: |
          curl -X POST https://your-beemflow-instance.com/cron
```

### 5. AWS EventBridge / CloudWatch Events

Create a rule that triggers a Lambda function or directly calls your BeemFlow endpoint.

## Auto-Setup (Server Mode)

When running BeemFlow in server mode, it can automatically manage system cron entries:

```bash
beemflow serve --auto-cron
```

This will:
1. Add cron entries for each workflow based on their `cron` field
2. Clean up entries on shutdown
3. Update entries when workflows change

## Best Practices

1. **Use Per-Workflow Endpoints** for precise scheduling control
2. **Monitor Failed Triggers** - Set up alerting on your cron system
3. **Idempotent Workflows** - Design workflows to handle duplicate triggers
4. **Time Zones** - Cron expressions typically use system time zone

## Security

For production:
- Use authentication tokens in headers
- Restrict endpoint access by IP
- Monitor for unusual trigger patterns

Example with auth:
```bash
*/5 * * * * curl -X POST -H "Authorization: Bearer $BEEMFLOW_TOKEN" http://localhost:3333/cron
```

## Future Enhancements

- Built-in scheduling UI
- Webhook signature verification  
- Schedule history and metrics
- Dynamic schedule updates via API