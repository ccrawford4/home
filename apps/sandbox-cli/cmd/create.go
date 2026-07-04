package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"sandbox-cli/api"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new sandbox",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := api.NewClient(log, apiURL)
		sandbox, err := c.Create(cmd.Context())
		if err != nil {
			return fmt.Errorf("create: %w", err)
		}
		fmt.Printf("Created sandbox %s\n", sandbox.ID)
		return nil
	},
}
