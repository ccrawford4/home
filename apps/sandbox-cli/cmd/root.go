package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var apiURL string

var rootCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "sandbox CLI — manage sandboxed execution environments",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:8080", "Sandbox API URL")
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listCmd)
}
