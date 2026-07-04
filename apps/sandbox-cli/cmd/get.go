package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"sandbox-cli/api"
)

var getCmd = &cobra.Command{
	Use:   "get <sandbox-id>",
	Short: "Get details of a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := api.NewClient(log, apiURL)
		sandbox, err := c.Get(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		fmt.Printf("ID: %s\nStatus: %s\n", sandbox.ID, sandbox.Status)
		return nil
	},
}
