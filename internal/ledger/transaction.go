package ledger

import (
	"fmt"
	"time"
)

type Entry struct {
	ID            int64     `json:"id,omitempty"`
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	Amount        int64     `json:"amount"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
}

type Transaction struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Entries     []Entry   `json:"entries"`
	Finalized   bool      `json:"finalized"`
	PostedAt    time.Time `json:"posted_at"`
}

// Validate checks transaction invariants: at least 2 entries, per-currency sum is zero.
func (t *Transaction) Validate() error {
	if t.Description == "" {
		return ErrEmptyDescription
	}
	if len(t.Entries) < 2 {
		return ErrTooFewEntries
	}

	byCurrency := make(map[string]int64)
	for _, e := range t.Entries {
		byCurrency[e.Currency] += e.Amount
	}
	for cur, sum := range byCurrency {
		if sum != 0 {
			return fmt.Errorf("%w: currency %s sums to %d", ErrUnbalancedTransaction, cur, sum)
		}
	}
	return nil
}

// BalanceSheet represents the balance sheet report.
type BalanceSheetLine struct {
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	Balance     int64  `json:"balance"`
	Currency    string `json:"currency"`
}

type BalanceSheet struct {
	Assets           []BalanceSheetLine `json:"assets"`
	Liabilities      []BalanceSheetLine `json:"liabilities"`
	Equity           []BalanceSheetLine `json:"equity"`
	TotalAssets      int64              `json:"total_assets"`
	TotalLiabilities int64              `json:"total_liabilities"`
	TotalEquity      int64              `json:"total_equity"`
	Balanced         bool               `json:"balanced"`
	GeneratedAt      time.Time          `json:"generated_at"`
}

// TrialBalanceLine represents a single line in the trial balance.
type TrialBalanceLine struct {
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	Debit       int64  `json:"debit"`
	Credit      int64  `json:"credit"`
	Currency    string `json:"currency"`
}

type TrialBalance struct {
	Lines       []TrialBalanceLine `json:"lines"`
	TotalDebit  int64              `json:"total_debit"`
	TotalCredit int64              `json:"total_credit"`
	Balanced    bool               `json:"balanced"`
	GeneratedAt time.Time          `json:"generated_at"`
}

// RegulatoryRatios holds key prudential metrics.
type RegulatoryRatios struct {
	CapitalAdequacy  float64 `json:"capital_adequacy"`
	LeverageRatio    float64 `json:"leverage_ratio"`
	ReserveRatio     float64 `json:"reserve_ratio"`
	Equity           int64   `json:"equity"`            // absolute value (negated from credit-normal)
	TotalAssets      int64   `json:"total_assets"`
	Reserves         int64   `json:"reserves"`           // code 1060 balances
	CustomerDeposits int64   `json:"customer_deposits"`  // absolute value (negated from credit-normal)
}

// ProjectRatios computes projected ratios after applying proposed entries.
// accounts maps account ID → Account for category/code lookup.
func ProjectRatios(current *RegulatoryRatios, entries []Entry, accounts map[string]Account) *RegulatoryRatios {
	projected := *current

	for _, e := range entries {
		acct, ok := accounts[e.AccountID]
		if !ok {
			continue
		}
		switch acct.Category {
		case CategoryAssets:
			projected.TotalAssets += e.Amount
		case CategoryEquity:
			// Equity stored as absolute; credit entries are negative amounts
			projected.Equity += -e.Amount
		case CategoryLiabilities:
			if acct.Code == 2020 {
				projected.CustomerDeposits += -e.Amount
			}
		}
		if acct.Code == 1060 {
			projected.Reserves += e.Amount
		}
	}

	if projected.TotalAssets > 0 {
		projected.CapitalAdequacy = float64(projected.Equity) / float64(projected.TotalAssets) * 100
		projected.LeverageRatio = projected.CapitalAdequacy
	}
	if projected.CustomerDeposits > 0 {
		projected.ReserveRatio = float64(projected.Reserves) / float64(projected.CustomerDeposits) * 100
	}

	return &projected
}
