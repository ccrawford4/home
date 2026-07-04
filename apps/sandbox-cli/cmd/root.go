package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"sandbox-cli/internal/logger"
)

var (
	apiURL string
	log    *slog.Logger
)

var rootCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "sandbox CLI — manage sandboxed execution environments",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		log = logger.New()
		slog.SetDefault(log)
	},
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
