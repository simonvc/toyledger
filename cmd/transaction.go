package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
	"github.com/spf13/cobra"
)

var transactionCmd = &cobra.Command{
	Use:     "transaction",
	Aliases: []string{"txn"},
	Short:   "Manage transactions",
}

// transaction create
var (
	txnDescription string
	txnEntries     []string // format: "account_id:amount:currency"
)

var transactionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new transaction",
	Long:  `Create a transaction with double-entry bookkeeping entries.\nEach --entry is formatted as "account_id:amount:currency" (e.g. "1010:+5000:USD")`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(flagServer)

		txn := &ledger.Transaction{
			Description: txnDescription,
		}

		for _, e := range txnEntries {
			parts := strings.SplitN(e, ":", 3)
			if len(parts) != 3 {
				return fmt.Errorf("invalid entry format %q, expected account_id:amount:currency", e)
			}
			amount, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid amount %q in entry %q: %w", parts[1], e, err)
			}
			txn.Entries = append(txn.Entries, ledger.Entry{
				AccountID: parts[0],
				Amount:    amount,
				Currency:  parts[2],
			})
		}

		created, err := c.CreateTransaction(context.Background(), txn)
		if err != nil {
			return err
		}

		fmt.Printf("Transaction created: %s\n", created.ID)
		fmt.Printf("Description: %s\n", created.Description)
		fmt.Printf("Entries:\n")
		for _, entry := range created.Entries {
			direction := "DR"
			amt := entry.Amount
			if amt < 0 {
				direction = "CR"
				amt = -amt
			}
			fmt.Printf("  %s %-12s %s %s\n", direction, entry.AccountID, ledger.FormatAmount(amt, entry.Currency), entry.Currency)
		}
		return nil
	},
}

// transaction list
var txnListAccountID string

var transactionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List transactions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(flagServer)

		txns, err := c.ListTransactions(context.Background(), txnListAccountID)
		if err != nil {
			return err
		}

		if len(txns) == 0 {
			fmt.Println("No transactions found.")
			return nil
		}

		fmt.Printf("%-38s %-20s %-8s %s\n", "ID", "DATE", "ENTRIES", "DESCRIPTION")
		fmt.Printf("%-38s %-20s %-8s %s\n", "----", "----", "-------", "-----------")
		for _, t := range txns {
			desc := t.Description
			if len(desc) > 40 {
				desc = desc[:38] + ".."
			}
			fmt.Printf("%-38s %-20s %-8d %s\n",
				t.ID,
				t.PostedAt.Format("2006-01-02 15:04"),
				len(t.Entries),
				desc,
			)
		}
		return nil
	},
}

// transaction get
var transactionGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get transaction details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(flagServer)

		txn, err := c.GetTransaction(context.Background(), args[0])
		if err != nil {
			return err
		}

		fmt.Printf("ID:          %s\n", txn.ID)
		fmt.Printf("Description: %s\n", txn.Description)
		fmt.Printf("Posted:      %s\n", txn.PostedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Finalized:   %v\n", txn.Finalized)
		fmt.Printf("Entries:\n")
		fmt.Printf("  %-4s %-12s %12s %s\n", "TYPE", "ACCOUNT", "AMOUNT", "CURRENCY")
		for _, entry := range txn.Entries {
			direction := "DR"
			amt := entry.Amount
			if amt < 0 {
				direction = "CR"
				amt = -amt
			}
			fmt.Printf("  %-4s %-12s %12s %s\n", direction, entry.AccountID, ledger.FormatAmount(amt, entry.Currency), entry.Currency)
		}
		return nil
	},
}

func init() {
	transactionCreateCmd.Flags().StringVar(&txnDescription, "description", "", "Transaction description")
	transactionCreateCmd.Flags().StringSliceVar(&txnEntries, "entry", nil, "Entry in format account_id:amount:currency (can be repeated)")
	transactionCreateCmd.MarkFlagRequired("description")
	transactionCreateCmd.MarkFlagRequired("entry")

	transactionListCmd.Flags().StringVar(&txnListAccountID, "account", "", "Filter by account ID")

	transactionCmd.AddCommand(transactionCreateCmd)
	transactionCmd.AddCommand(transactionListCmd)
	transactionCmd.AddCommand(transactionGetCmd)

	rootCmd.AddCommand(transactionCmd)
}
