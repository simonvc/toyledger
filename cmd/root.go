package cmd

import (
	"github.com/spf13/cobra"
)

var (
	flagServer string
	flagDB     string
)

var rootCmd = &cobra.Command{
	Use:   "miniledger",
	Short: "Double-entry accounting ledger with IFRS chart of accounts",
	Long:  "A double-entry accounting ledger backed by SQLite, supporting multi-currency transactions and IFRS chart of accounts.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagServer, "server", "http://localhost:8888", "Server address")
	rootCmd.PersistentFlags().StringVar(&flagDB, "db", "ledger.db", "SQLite database path")
}

func Execute() error {
	return rootCmd.Execute()
}
