package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/ledger"
	"github.com/spf13/cobra"
)

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Show balance sheet",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(flagServer)

		bs, err := c.BalanceSheet(context.Background())
		if err != nil {
			return err
		}

		printBalanceSheet(bs)
		return nil
	},
}

var trialBalanceCmd = &cobra.Command{
	Use:   "trial",
	Short: "Show trial balance",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(flagServer)

		tb, err := c.TrialBalance(context.Background())
		if err != nil {
			return err
		}

		printTrialBalance(tb)
		return nil
	},
}

func printBalanceSheet(bs *ledger.BalanceSheet) {
	w := 60
	fmt.Println()
	fmt.Println(center("BALANCE SHEET", w))
	fmt.Println(center(strings.Repeat("=", 20), w))
	fmt.Println()

	printSection("ASSETS", bs.Assets, w)
	fmt.Printf("%*s%s\n", w-15, "", "─────────────")
	fmt.Printf("%-*s%15s\n", w-15, "Total Assets", formatSigned(bs.TotalAssets, "USD"))
	fmt.Println()

	printSection("LIABILITIES", bs.Liabilities, w)
	fmt.Printf("%*s%s\n", w-15, "", "─────────────")
	fmt.Printf("%-*s%15s\n", w-15, "Total Liabilities", formatSigned(bs.TotalLiabilities, "USD"))
	fmt.Println()

	printSection("EQUITY", bs.Equity, w)
	fmt.Printf("%*s%s\n", w-15, "", "─────────────")
	fmt.Printf("%-*s%15s\n", w-15, "Total Equity", formatSigned(bs.TotalEquity, "USD"))
	fmt.Println()

	fmt.Printf("%*s%s\n", w-15, "", "═════════════")
	fmt.Printf("%-*s%15s\n", w-15, "Total L + E", formatSigned(bs.TotalLiabilities+bs.TotalEquity, "USD"))

	if bs.Balanced {
		fmt.Println("\n  [BALANCED]")
	} else {
		fmt.Println("\n  [UNBALANCED!]")
	}
}

func printSection(title string, lines []ledger.BalanceSheetLine, w int) {
	fmt.Printf("  %s\n", title)
	fmt.Printf("  %s\n", strings.Repeat("─", w-4))
	for _, l := range lines {
		name := l.AccountName
		if len(name) > 30 {
			name = name[:28] + ".."
		}
		fmt.Printf("  %-6s %-*s%15s\n", l.AccountID, w-24, name, formatSigned(l.Balance, l.Currency))
	}
}

func printTrialBalance(tb *ledger.TrialBalance) {
	w := 70
	fmt.Println()
	fmt.Println(center("TRIAL BALANCE", w))
	fmt.Println(center(strings.Repeat("=", 20), w))
	fmt.Println()

	fmt.Printf("  %-8s %-30s %15s %15s\n", "ID", "NAME", "DEBIT", "CREDIT")
	fmt.Printf("  %-8s %-30s %15s %15s\n", "----", "----", "-----", "------")

	for _, l := range tb.Lines {
		name := l.AccountName
		if len(name) > 28 {
			name = name[:28] + ".."
		}
		debit := ""
		credit := ""
		if l.Debit > 0 {
			debit = ledger.FormatAmount(l.Debit, l.Currency)
		}
		if l.Credit > 0 {
			credit = ledger.FormatAmount(l.Credit, l.Currency)
		}
		fmt.Printf("  %-8s %-30s %15s %15s\n", l.AccountID, name, debit, credit)
	}

	fmt.Printf("  %s\n", strings.Repeat("─", w-4))
	fmt.Printf("  %-39s %15s %15s\n", "TOTALS",
		ledger.FormatAmount(tb.TotalDebit, "USD"),
		ledger.FormatAmount(tb.TotalCredit, "USD"))

	if tb.Balanced {
		fmt.Println("\n  [BALANCED]")
	} else {
		fmt.Println("\n  [UNBALANCED!]")
	}
}

func center(s string, w int) string {
	if len(s) >= w {
		return s
	}
	pad := (w - len(s)) / 2
	return strings.Repeat(" ", pad) + s
}

func formatSigned(amount int64, currency string) string {
	if amount < 0 {
		return "(" + ledger.FormatAmount(-amount, currency) + ")"
	}
	return ledger.FormatAmount(amount, currency)
}

func init() {
	balanceCmd.AddCommand(trialBalanceCmd)
	rootCmd.AddCommand(balanceCmd)
}
