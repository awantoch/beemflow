package main

import (
	"bufio"
	"context"
	"os"
	"strings"

	"github.com/awantoch/beemflow/adapter"
	"github.com/awantoch/beemflow/utils/logger"
	"github.com/spf13/cobra"
)

func newAssistCmd() *cobra.Command {
	var prompt string
	var output string

	cmd := &cobra.Command{
		Use:   "assist",
		Short: "Interactively draft, refine, and validate flows with the BeemFlow assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			var messages []string

			if prompt != "" {
				// One-shot mode
				messages = append(messages, prompt)
			} else {
				// Interactive REPL mode
				scanner := bufio.NewScanner(os.Stdin)
				logger.User("BeemFlow Assistant (type 'exit' to quit)")
				for {
					logger.User("user> ")
					if !scanner.Scan() {
						break
					}
					line := scanner.Text()
					if strings.TrimSpace(line) == "exit" {
						break
					}
					messages = append(messages, line)
					// Call assistant after each message
					draft, errors, err := adapter.Execute(ctx, messages)
					if err != nil {
						logger.Error("Assistant error: %v", err)
						continue
					}
					logger.User("\n--- Draft Flow ---\n%s", draft)
					if len(errors) > 0 {
						logger.User("\n--- Validation Errors ---")
						for _, e := range errors {
							logger.User("- %v", e)
						}
					}
				}
				return nil
			}

			draft, errors, err := adapter.Execute(ctx, messages)
			if err != nil {
				return err
			}
			if output != "" {
				f, err := os.Create(output)
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = f.WriteString(draft)
				if err != nil {
					return err
				}
				logger.User("Draft written to %s", output)
			} else {
				logger.User("\n--- Draft Flow ---\n%s", draft)
			}
			if len(errors) > 0 {
				logger.User("\n--- Validation Errors ---")
				for _, e := range errors {
					logger.User("- %v", e)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "One-shot prompt for the assistant")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write draft flow to file")
	return cmd
}
