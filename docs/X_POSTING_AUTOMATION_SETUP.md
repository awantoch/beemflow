# X Posting Automation Setup Guide

This guide explains how to set up an automated workflow for posting to X (Twitter) using Google Drive for content uploads and Google Sheets for team approval.

## Overview

The workflow automates the following process:
1. **Content Upload**: Team members upload content to a shared Google Drive folder
2. **Draft Generation**: AI generates tweet drafts from the uploaded content
3. **Approval Process**: Drafts are added to a Google Sheet for team review
4. **Automated Posting**: Approved tweets are automatically posted to X

## Prerequisites

1. BeemFlow runtime installed and configured
2. Google Cloud Service Account with access to:
   - Google Drive API
   - Google Sheets API
3. X (Twitter) Developer Account with API credentials
4. OpenAI API key (for draft generation)

## Configuration

### 1. Environment Variables

Create or update your `.env` file with the following:

```bash
# Google Workspace
GOOGLE_API_KEY=your_google_api_key
GOOGLE_SERVICE_ACCOUNT_JSON=/path/to/service-account.json
GOOGLE_DRIVE_FOLDER_ID=your_drive_folder_id
GOOGLE_SHEETS_DRAFT_ID=your_sheets_document_id

# X (Twitter) API
TWITTER_API_KEY=your_api_key
TWITTER_API_SECRET=your_api_secret
TWITTER_ACCESS_TOKEN=your_access_token
TWITTER_ACCESS_SECRET=your_access_secret

# OpenAI
OPENAI_API_KEY=your_openai_api_key
```

### 2. Google Sheets Setup

Create a Google Sheet with the following columns:
- **A**: Timestamp (auto-filled)
- **B**: Source File
- **C**: Draft Tweet
- **D**: Status (Pending/Posted)
- **E**: Approved (checkbox)
- **F**: Posted Timestamp

Share this sheet with your service account email.

### 3. MCP Server Configuration

Add the following to your `flow.config.json`:

```json
{
  "mcpServers": {
    "google-sheets": {
      "command": "npx",
      "args": ["-y", "@xing5/mcp-server-google-sheets"],
      "env": {
        "GOOGLE_API_KEY": "$env:GOOGLE_API_KEY",
        "GOOGLE_SERVICE_ACCOUNT_JSON": "$env:GOOGLE_SERVICE_ACCOUNT_JSON"
      }
    },
    "x-twitter": {
      "command": "npx",
      "args": ["-y", "@profullstack/x-twitter-mcp"],
      "env": {
        "TWITTER_API_KEY": "$env:TWITTER_API_KEY",
        "TWITTER_API_SECRET": "$env:TWITTER_API_SECRET",
        "TWITTER_ACCESS_TOKEN": "$env:TWITTER_ACCESS_TOKEN",
        "TWITTER_ACCESS_SECRET": "$env:TWITTER_ACCESS_SECRET"
      }
    }
  }
}
```

## Setting Up Cron Jobs

### 1. Google Drive Monitoring (Every 10 minutes)

```bash
# Add to crontab
*/10 * * * * /path/to/beemflow/scripts/poll_google_drive.sh
```

### 2. Approval Checking (Every 5 minutes)

```bash
# Add to crontab
*/5 * * * * /path/to/beemflow/scripts/poll_x_approvals.sh
```

Make the scripts executable:
```bash
chmod +x scripts/poll_google_drive.sh
chmod +x scripts/poll_x_approvals.sh
```

## Usage

### 1. Starting the BeemFlow Runtime

```bash
# Start the BeemFlow server
make run

# Or in development mode
make dev
```

### 2. Team Workflow

1. **Content Creation**: Team members upload content files to the designated Google Drive folder
2. **Review Drafts**: Check the Google Sheet for new draft tweets
3. **Edit & Approve**: Team can edit the tweet text and check the "Approved" box
4. **Automatic Posting**: The system will post approved tweets within 5 minutes

### 3. Manual Triggers

You can manually trigger workflows:

```bash
# Trigger approval check
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{"topic": "tweet.check_approvals", "data": {}}'

# Simulate new file upload
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "google.drive.new_file",
    "data": {
      "fileId": "test_file_id",
      "fileName": "test_content.txt"
    }
  }'
```

## Monitoring

### Viewing Logs

```bash
# View BeemFlow logs
tail -f .beemflow/logs/beemflow.log

# View specific workflow execution
beemflow logs --run-id <run-id>
```

### Checking Workflow Status

```bash
# List recent runs
beemflow list runs --limit 10

# Get details of a specific run
beemflow get run <run-id>
```

## Security Best Practices

1. **Service Account**: Use a dedicated Google Service Account with minimal permissions
2. **API Keys**: Store all API keys securely and never commit them to version control
3. **Sheet Permissions**: Only share the Google Sheet with authorized team members
4. **Rate Limiting**: Be mindful of API rate limits for both Google and X APIs

## Troubleshooting

### Common Issues

1. **Authentication Errors**: 
   - Verify service account JSON path
   - Check that APIs are enabled in Google Cloud Console
   - Ensure service account has access to sheets/drive

2. **MCP Server Not Found**:
   - Run `npm install -g @xing5/mcp-server-google-sheets`
   - Check that npx can find the package

3. **Tweets Not Posting**:
   - Verify X API credentials
   - Check approval status in Google Sheet
   - Review logs for API errors

### Debug Mode

Enable debug logging:
```bash
export BEEMFLOW_LOG_LEVEL=debug
make run
```

## Advanced Configuration

### Custom Tweet Templates

Modify the AI prompt in `x_posting_automation.flow.yaml`:

```yaml
- role: system
  content: |
    You are a social media expert. Create an engaging tweet (max 280 chars) 
    based on the provided content. Include relevant hashtags.
    Brand voice: [Your brand voice description]
    Always include: @YourHandle
```

### Multiple Approval Levels

Add additional columns to your Google Sheet:
- **G**: Reviewer 1 Approved
- **H**: Reviewer 2 Approved

Then modify the approval check in `x_posting_check_approvals.flow.yaml`:

```yaml
when: "{{ row.4 == 'TRUE' && row.6 == 'TRUE' && row.7 == 'TRUE' && row.5 == '' }}"
```

## Support

For issues or questions:
1. Check the BeemFlow documentation
2. Review workflow logs
3. Submit issues to the BeemFlow repository