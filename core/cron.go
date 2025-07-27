package api

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/utils"
	"github.com/robfig/cron/v3"
)

const cronMarker = "# BeemFlow managed - do not edit"

// ShellQuote safely quotes a string for use in shell commands
// It escapes single quotes and wraps the string in single quotes
// This prevents shell injection attacks
func ShellQuote(s string) string {
	// Replace single quotes with '\'' (end quote, escaped quote, start quote)
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	// Wrap in single quotes
	return "'" + escaped + "'"
}

// shellQuote is the internal version
func shellQuote(s string) string {
	return ShellQuote(s)
}

// CronManager handles system cron integration
type CronManager struct {
	serverURL  string
	cronSecret string
}

// NewCronManager creates a new cron manager
func NewCronManager(serverURL string, cronSecret string) *CronManager {
	return &CronManager{
		serverURL:  serverURL,
		cronSecret: cronSecret,
	}
}

// SyncCronEntries updates system cron with workflow schedules
func (c *CronManager) SyncCronEntries(ctx context.Context) error {
	// Get all workflows with cron schedules
	flows, err := ListFlows(ctx)
	if err != nil {
		return err
	}

	var entries []string
	for _, flowName := range flows {
		flow, err := GetFlow(ctx, flowName)
		if err != nil {
			continue
		}

		cronExpr := extractCronExpression(&flow)
		if cronExpr != "" {
			// Build curl command with proper escaping to prevent injection
			var curlCmd strings.Builder
			curlCmd.WriteString("curl -sS -X POST")
			
			// Add authorization header if CRON_SECRET is set
			if c.cronSecret != "" {
				// Properly escape the secret in the header
				curlCmd.WriteString(" -H ")
				curlCmd.WriteString(shellQuote("Authorization: Bearer " + c.cronSecret))
			}
			
			// Build URL with proper escaping and URL encoding
			encodedFlowName := url.PathEscape(flowName)
			fullURL := fmt.Sprintf("%s/cron/%s", c.serverURL, encodedFlowName)
			curlCmd.WriteString(" ")
			curlCmd.WriteString(shellQuote(fullURL))
			curlCmd.WriteString(" >/dev/null 2>&1")
			
			// Create cron entry with proper spacing
			entry := fmt.Sprintf("%s %s %s", cronExpr, curlCmd.String(), cronMarker)
			entries = append(entries, entry)
		}
	}

	return c.updateSystemCron(entries)
}

// updateSystemCron updates the system crontab
func (c *CronManager) updateSystemCron(newEntries []string) error {
	// Get current crontab
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.Output()
	if err != nil {
		// No existing crontab is okay
		output = []byte{}
	}

	// Filter out our managed entries
	var preservedLines []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, cronMarker) {
			preservedLines = append(preservedLines, line)
		}
	}

	// Add new entries
	allLines := append(preservedLines, newEntries...)
	
	// Write back to crontab
	newCron := strings.Join(allLines, "\n") + "\n"
	cmd = exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCron)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update crontab: %w", err)
	}

	utils.Info("Updated system cron with %d BeemFlow entries", len(newEntries))
	return nil
}

// RemoveAllEntries removes all BeemFlow managed cron entries
func (c *CronManager) RemoveAllEntries() error {
	// Get current crontab
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil // No crontab, nothing to remove
	}

	// Filter out our managed entries
	var preservedLines []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, cronMarker) {
			preservedLines = append(preservedLines, line)
		}
	}

	// Write back
	newCron := strings.Join(preservedLines, "\n") + "\n"
	cmd = exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCron)
	
	return cmd.Run()
}

// extractCronExpression gets cron from flow (reuse existing logic)
func extractCronExpression(flow *model.Flow) string {
	if !hasScheduleCronTrigger(flow) || flow.Cron == "" {
		return ""
	}
	return flow.Cron
}

// CheckAndExecuteCronFlows checks all flows for cron schedules and executes those that are due
// This is optimized for serverless - it's stateless and relies only on the database
func CheckAndExecuteCronFlows(ctx context.Context) (map[string]interface{}, error) {
	// List all flows
	flows, err := ListFlows(ctx)
	if err != nil {
		return nil, err
	}
	
	triggered := []string{}
	errors := []string{}
	checked := 0
	
	// Get current time
	now := time.Now().UTC()
	
	// Create cron parser - using standard cron format (5 fields)
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	
	for _, flowName := range flows {
		flow, err := GetFlow(ctx, flowName)
		if err != nil {
			errors = append(errors, flowName + ": " + err.Error())
			continue
		}
		
		// Check if flow has schedule.cron trigger
		if !hasScheduleCronTrigger(&flow) {
			continue
		}
		
		checked++
		
		// Parse cron expression
		if flow.Cron == "" {
			errors = append(errors, flowName + ": missing cron expression")
			continue
		}
		
		schedule, err := parser.Parse(flow.Cron)
		if err != nil {
			errors = append(errors, flowName + ": invalid cron: " + err.Error())
			continue
		}
		
		// Check if we should run now
		// In serverless, we need to check if the schedule matches within our check window
		// Vercel cron runs every 5 minutes, so we check a 5-minute window
		if shouldRunNow(schedule, now, 5*time.Minute) {
			// Trigger the workflow
			event := map[string]interface{}{
				"trigger":   "schedule.cron",
				"workflow":  flowName,
				"timestamp": now.Format(time.RFC3339),
			}
			
			if _, err := StartRun(ctx, flowName, event); err != nil {
				errors = append(errors, flowName + ": failed to start: " + err.Error())
			} else {
				triggered = append(triggered, flowName)
				utils.Info("Triggered cron workflow: %s", flowName)
			}
		}
	}
	
	return map[string]interface{}{
		"status":    "completed",
		"timestamp": now.Format(time.RFC3339),
		"triggered": len(triggered),
		"workflows": triggered,
		"errors":    errors,
		"checked":   checked,
		"total":     len(flows),
	}, nil
}

// shouldRunNow checks if a cron schedule should run within the given window
// This handles the fact that Vercel cron might not run exactly on time
func shouldRunNow(schedule cron.Schedule, now time.Time, window time.Duration) bool {
	// Get the previous scheduled time
	// We look back one window period to find when it should have last run
	checkFrom := now.Add(-window)
	
	// Get when it should next run after our check start time
	nextRun := schedule.Next(checkFrom)
	
	// If the next run time is in the past (or within 1 minute future), we should run it
	// The 1 minute future buffer handles edge cases where Vercel cron is slightly early
	return nextRun.Before(now.Add(1 * time.Minute))
}

// hasScheduleCronTrigger checks if a flow has schedule.cron in its triggers
func hasScheduleCronTrigger(flow *model.Flow) bool {
	switch on := flow.On.(type) {
	case string:
		return on == "schedule.cron"
	case []interface{}:
		for _, trigger := range on {
			if str, ok := trigger.(string); ok && str == "schedule.cron" {
				return true
			}
		}
	}
	return false
}