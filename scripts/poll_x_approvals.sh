#!/bin/bash

# X Posting Approval Polling Script
# This script should be run via cron to periodically check for approved tweets
# Example cron entry (every 5 minutes):
# */5 * * * * /path/to/beemflow/scripts/poll_x_approvals.sh

# Set the BeemFlow directory
BEEMFLOW_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Load environment variables if .env exists
if [ -f "$BEEMFLOW_DIR/.env" ]; then
    export $(cat "$BEEMFLOW_DIR/.env" | grep -v '^#' | xargs)
fi

# Emit event to trigger approval checking
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "tweet.check_approvals",
    "data": {
      "source": "cron",
      "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
    }
  }'

echo "Triggered tweet approval check at $(date)"