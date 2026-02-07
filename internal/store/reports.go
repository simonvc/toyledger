package store

import (
	"context"
	"fmt"
	"time"

	"github.com/simonvc/miniledger/internal/ledger"
)

func (s *Store) AccountBalance(ctx context.Context, accountID string) (int64, string, error) {
	// Verify account exists
	acct, err := s.GetAccount(ctx, accountID)
	if err != nil {
		return 0, "", err
	}

	var balance int64
	err = s.reader.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(e.amount), 0)
		FROM entries e
		JOIN transactions t ON t.id = e.transaction_id
		WHERE e.account_id = ? AND t.finalized = 1`, accountID,
	).Scan(&balance)
	if err != nil {
		return 0, "", fmt.Errorf("account balance: %w", err)
	}

	return balance, acct.Currency, nil
}

func (s *Store) BalanceSheet(ctx context.Context) (*ledger.BalanceSheet, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT a.id, a.name, a.category, a.currency, COALESCE(SUM(e.amount), 0) as balance
		FROM accounts a
		LEFT JOIN entries e ON e.account_id = a.id
		LEFT JOIN transactions t ON t.id = e.transaction_id AND t.finalized = 1
		GROUP BY a.id
		HAVING balance != 0
		ORDER BY a.code`)
	if err != nil {
		return nil, fmt.Errorf("balance sheet query: %w", err)
	}
	defer rows.Close()

	bs := &ledger.BalanceSheet{
		GeneratedAt: time.Now().UTC(),
	}

	for rows.Next() {
		var line ledger.BalanceSheetLine
		var category string
		if err := rows.Scan(&line.AccountID, &line.AccountName, &category, &line.Currency, &line.Balance); err != nil {
			return nil, fmt.Errorf("scan balance sheet: %w", err)
		}

		// Skip wildcard-currency accounts (e.g. ~fx) — their balance is a
		// meaningless sum across currencies. The positions view handles them.
		if line.Currency == "*" {
			continue
		}

		gelAmt := ledger.ToGEL(line.Balance, line.Currency)
		switch ledger.Category(category) {
		case ledger.CategoryAssets:
			bs.Assets = append(bs.Assets, line)
			bs.TotalAssets += gelAmt
		case ledger.CategoryLiabilities:
			bs.Liabilities = append(bs.Liabilities, line)
			bs.TotalLiabilities += gelAmt
		case ledger.CategoryEquity:
			bs.Equity = append(bs.Equity, line)
			bs.TotalEquity += gelAmt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Totals are in reporting currency (GEL). Check the accounting equation:
	// Assets + Liabilities + Equity = 0 (liabilities/equity are credit-normal, stored negative)
	bs.Balanced = (bs.TotalAssets + bs.TotalLiabilities + bs.TotalEquity) == 0

	return bs, nil
}

func (s *Store) RegulatoryRatios(ctx context.Context) (*ledger.RegulatoryRatios, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT a.category, a.code, COALESCE(SUM(e.amount), 0) as balance
		FROM accounts a
		LEFT JOIN entries e ON e.account_id = a.id
		LEFT JOIN transactions t ON t.id = e.transaction_id AND t.finalized = 1
		GROUP BY a.category, a.code
		HAVING balance != 0`)
	if err != nil {
		return nil, fmt.Errorf("regulatory ratios query: %w", err)
	}
	defer rows.Close()

	r := &ledger.RegulatoryRatios{}
	for rows.Next() {
		var category string
		var code int
		var balance int64
		if err := rows.Scan(&category, &code, &balance); err != nil {
			return nil, fmt.Errorf("scan regulatory ratios: %w", err)
		}

		switch ledger.Category(category) {
		case ledger.CategoryAssets:
			r.TotalAssets += balance
			if code == 1060 {
				r.Reserves += balance
			}
		case ledger.CategoryEquity:
			r.Equity += -balance // negate: credit-normal → positive
		case ledger.CategoryLiabilities:
			if code == 2020 {
				r.CustomerDeposits += -balance // negate: credit-normal → positive
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if r.TotalAssets > 0 {
		r.CapitalAdequacy = float64(r.Equity) / float64(r.TotalAssets) * 100
		r.LeverageRatio = r.CapitalAdequacy
	}
	if r.CustomerDeposits > 0 {
		r.ReserveRatio = float64(r.Reserves) / float64(r.CustomerDeposits) * 100
	}

	return r, nil
}

func (s *Store) TrialBalance(ctx context.Context) (*ledger.TrialBalance, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT a.id, a.name, a.currency, COALESCE(SUM(e.amount), 0) as balance
		FROM accounts a
		LEFT JOIN entries e ON e.account_id = a.id
		LEFT JOIN transactions t ON t.id = e.transaction_id AND t.finalized = 1
		GROUP BY a.id
		HAVING balance != 0
		ORDER BY a.code`)
	if err != nil {
		return nil, fmt.Errorf("trial balance query: %w", err)
	}
	defer rows.Close()

	tb := &ledger.TrialBalance{
		GeneratedAt: time.Now().UTC(),
	}

	for rows.Next() {
		var line ledger.TrialBalanceLine
		var balance int64
		if err := rows.Scan(&line.AccountID, &line.AccountName, &line.Currency, &balance); err != nil {
			return nil, fmt.Errorf("scan trial balance: %w", err)
		}

		if balance > 0 {
			line.Debit = balance
			tb.TotalDebit += balance
		} else {
			line.Credit = -balance
			tb.TotalCredit += -balance
		}

		tb.Lines = append(tb.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tb.Balanced = tb.TotalDebit == tb.TotalCredit
	return tb, nil
}
