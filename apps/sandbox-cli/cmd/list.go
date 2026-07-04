package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"sandbox-cli/internal/client"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sandboxes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(apiURL)
		sandboxes, err := c.List()
		if err != nil {
			return fmt.Errorf("list: %w", err)
		}
		if len(sandboxes) == 0 {
			fmt.Println("No sandboxes found.")
			return nil
		}
		for _, s := range sandboxes {
			fmt.Printf("%s\t%s\n", s.ID, s.Status)
		}
		return nil
	},
}
