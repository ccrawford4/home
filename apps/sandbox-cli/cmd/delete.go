package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"sandbox-cli/internal/client"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <sandbox-id>",
	Short: "Delete a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(apiURL)
		if err := c.Delete(args[0]); err != nil {
			return fmt.Errorf("delete: %w", err)
		}
		fmt.Printf("Deleted sandbox %s\n", args[0])
		return nil
	},
}
