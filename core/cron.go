package api

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/awantoch/beemflow/model"
	"github.com/awantoch/beemflow/utils"
)

const cronMarker = "# BeemFlow managed - do not edit"

// CronManager handles system cron integration
type CronManager struct {
	serverURL string
}

// NewCronManager creates a new cron manager
func NewCronManager(serverURL string) *CronManager {
	return &CronManager{
		serverURL: serverURL,
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
			// Create cron entry
			entry := fmt.Sprintf("%s curl -sS -X POST %s/cron/%s %s",
				cronExpr,
				c.serverURL,
				flowName,
				cronMarker)
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
	// Check if triggered by schedule.cron
	hasScheduleCron := false
	switch on := flow.On.(type) {
	case string:
		hasScheduleCron = (on == "schedule.cron")
	case []interface{}:
		for _, trigger := range on {
			if str, ok := trigger.(string); ok && str == "schedule.cron" {
				hasScheduleCron = true
				break
			}
		}
	}

	if !hasScheduleCron || flow.Cron == "" {
		return ""
	}

	return flow.Cron
}