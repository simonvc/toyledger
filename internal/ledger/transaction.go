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
