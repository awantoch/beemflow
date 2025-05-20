package main

import (
	"fmt"

	"github.com/awantoch/beemflow/docs"
	"github.com/spf13/cobra"
)

func newSpecCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "spec",
		Short: "Show the BeemFlow protocol & specification",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(docs.BeemflowSpec)
		},
	}
}
