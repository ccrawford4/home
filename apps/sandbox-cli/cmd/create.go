package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"sandbox-cli/internal/client"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new sandbox",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(apiURL)
		sandbox, err := c.Create()
		if err != nil {
			return fmt.Errorf("create: %w", err)
		}
		fmt.Printf("Created sandbox %s\n", sandbox.ID)
		return nil
	},
}
