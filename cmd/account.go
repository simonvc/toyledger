package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
	"github.com/spf13/cobra"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage accounts",
}

// account create
var (
	acctCreateID       string
	acctCreateName     string
	acctCreateCode     int
	acctCreateCurrency string
)

var accountCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new account",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(flagServer)

		acct := &ledger.Account{
			ID:       acctCreateID,
			Name:     acctCreateName,
			Code:     acctCreateCode,
			Currency: acctCreateCurrency,
		}

		created, err := c.CreateAccount(context.Background(), acct)
		if err != nil {
			return err
		}

		fmt.Printf("Account created: %s (%s) [%d] %s %s\n",
			created.ID, created.Name, created.Code, created.Category, created.Currency)
		return nil
	},
}

// account list
var acctListCategory string

var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(flagServer)

		accounts, err := c.ListAccounts(context.Background(), acctListCategory, nil)
		if err != nil {
			return err
		}

		if len(accounts) == 0 {
			fmt.Println("No accounts found.")
			return nil
		}

		fmt.Printf("%-12s %-30s %6s %-15s %s\n", "ID", "NAME", "CODE", "CATEGORY", "CURRENCY")
		fmt.Printf("%-12s %-30s %6s %-15s %s\n", "----", "----", "----", "--------", "--------")
		for _, a := range accounts {
			name := a.Name
			if len(name) > 28 {
				name = name[:28] + ".."
			}
			fmt.Printf("%-12s %-30s %6d %-15s %s\n", a.ID, name, a.Code, a.Category, a.Currency)
		}
		return nil
	},
}

// account get
var accountGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get account details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(flagServer)

		acct, err := c.GetAccount(context.Background(), args[0])
		if err != nil {
			return err
		}

		fmt.Printf("ID:       %s\n", acct.ID)
		fmt.Printf("Name:     %s\n", acct.Name)
		fmt.Printf("Code:     %d\n", acct.Code)
		fmt.Printf("Category: %s\n", acct.Category)
		fmt.Printf("Currency: %s\n", acct.Currency)
		fmt.Printf("System:   %v\n", acct.IsSystem)
		fmt.Printf("Created:  %s\n", acct.CreatedAt.Format("2006-01-02 15:04:05"))
		return nil
	},
}

// account balance
var accountBalanceCmd = &cobra.Command{
	Use:   "balance [id]",
	Short: "Get account balance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(flagServer)

		bal, err := c.GetAccountBalance(context.Background(), args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Account: %s\n", bal.AccountID)
		fmt.Printf("Balance: %s %s (%s minor units)\n", bal.Formatted, bal.Currency, strconv.FormatInt(bal.Balance, 10))
		return nil
	},
}

func init() {
	accountCreateCmd.Flags().StringVar(&acctCreateID, "id", "", "Account ID (e.g. 1010, ~fees)")
	accountCreateCmd.Flags().StringVar(&acctCreateName, "name", "", "Account name")
	accountCreateCmd.Flags().IntVar(&acctCreateCode, "code", 0, "IFRS account code")
	accountCreateCmd.Flags().StringVar(&acctCreateCurrency, "currency", "USD", "Currency (ISO 4217)")
	accountCreateCmd.MarkFlagRequired("id")
	accountCreateCmd.MarkFlagRequired("name")
	accountCreateCmd.MarkFlagRequired("code")

	accountListCmd.Flags().StringVar(&acctListCategory, "category", "", "Filter by category")

	accountCmd.AddCommand(accountCreateCmd)
	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountGetCmd)
	accountCmd.AddCommand(accountBalanceCmd)

	rootCmd.AddCommand(accountCmd)
}
