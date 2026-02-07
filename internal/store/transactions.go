package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/simonvc/miniledger/internal/ledger"
)

func (s *Store) CreateTransaction(ctx context.Context, txn *ledger.Transaction) error {
	if txn.ID == "" {
		txn.ID = uuid.Must(uuid.NewV7()).String()
	}
	if txn.PostedAt.IsZero() {
		txn.PostedAt = time.Now().UTC()
	}

	if err := txn.Validate(); err != nil {
		return err
	}

	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Insert transaction (finalized=0)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO transactions (id, description, posted_at) VALUES (?, ?, ?)`,
		txn.ID, txn.Description, txn.PostedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}

	// Collect account codes for settings enforcement
	accountCodes := map[string]int{}    // account_id -> code
	accountCats := map[string]string{}  // account_id -> category
	for _, e := range txn.Entries {
		if _, ok := accountCodes[e.AccountID]; !ok {
			var code int
			var cat string
			err := tx.QueryRowContext(ctx,
				`SELECT code, category FROM accounts WHERE id = ?`, e.AccountID).Scan(&code, &cat)
			if err != nil {
				return fmt.Errorf("lookup account %s: %w", e.AccountID, err)
			}
			accountCodes[e.AccountID] = code
			accountCats[e.AccountID] = cat
		}
	}

	// Load settings for all involved codes
	codeSettingsCache := map[int]ledger.CodeSettings{}
	for _, code := range accountCodes {
		if _, ok := codeSettingsCache[code]; !ok {
			var cs ledger.CodeSettings
			cs = ledger.DefaultCodeSettings(code)
			rows, qerr := tx.QueryContext(ctx,
				`SELECT setting, value FROM coa_settings WHERE code = ?`, code)
			if qerr != nil {
				return fmt.Errorf("load code settings %d: %w", code, qerr)
			}
			for rows.Next() {
				var name ledger.SettingName
				var value string
				if err := rows.Scan(&name, &value); err != nil {
					rows.Close()
					return fmt.Errorf("scan code setting: %w", err)
				}
				switch name {
				case ledger.SettingBlockInverted:
					cs.BlockInverted = value == "1"
				case ledger.SettingEntryDirection:
					cs.EntryDirection = ledger.EntryDirection(value)
				}
			}
			rows.Close()
			codeSettingsCache[code] = cs
		}
	}

	// Insert all entries (with entry direction check)
	for i := range txn.Entries {
		txn.Entries[i].TransactionID = txn.ID
		code := accountCodes[txn.Entries[i].AccountID]
		cs := codeSettingsCache[code]

		// Entry direction enforcement
		if cs.EntryDirection == ledger.DirectionDebitOnly && txn.Entries[i].Amount < 0 {
			return fmt.Errorf("%w: account %s (code %d) only allows debit entries",
				ledger.ErrEntryDirectionViolation, txn.Entries[i].AccountID, code)
		}
		if cs.EntryDirection == ledger.DirectionCreditOnly && txn.Entries[i].Amount > 0 {
			return fmt.Errorf("%w: account %s (code %d) only allows credit entries",
				ledger.ErrEntryDirectionViolation, txn.Entries[i].AccountID, code)
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO entries (transaction_id, account_id, amount, currency) VALUES (?, ?, ?, ?)`,
			txn.ID, txn.Entries[i].AccountID, txn.Entries[i].Amount, txn.Entries[i].Currency,
		)
		if err != nil {
			return fmt.Errorf("insert entry %d: %w", i, err)
		}
	}

	// Block inverted balance check (before finalization)
	for acctID, code := range accountCodes {
		cs := codeSettingsCache[code]
		if !cs.BlockInverted {
			continue
		}

		// Compute projected balance: existing finalized + this txn's entries
		var existingBalance int64
		err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(e.amount), 0) FROM entries e
			 JOIN transactions t ON t.id = e.transaction_id AND t.finalized = 1
			 WHERE e.account_id = ?`, acctID).Scan(&existingBalance)
		if err != nil {
			return fmt.Errorf("check balance for %s: %w", acctID, err)
		}

		var txnAmount int64
		for _, e := range txn.Entries {
			if e.AccountID == acctID {
				txnAmount += e.Amount
			}
		}

		projected := existingBalance + txnAmount
		cat := accountCats[acctID]
		isDebitNormal := cat == "assets" || cat == "expenses"

		if isDebitNormal && projected < 0 {
			return fmt.Errorf("%w: account %s (code %d) would have negative balance %d",
				ledger.ErrInvertedBalance, acctID, code, projected)
		}
		if !isDebitNormal && projected > 0 {
			return fmt.Errorf("%w: account %s (code %d) would have positive balance %d",
				ledger.ErrInvertedBalance, acctID, code, projected)
		}
	}

	// Finalize - trigger fires to validate balance
	_, err = tx.ExecContext(ctx,
		`UPDATE transactions SET finalized = 1 WHERE id = ?`, txn.ID)
	if err != nil {
		return fmt.Errorf("finalize transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	txn.Finalized = true
	return nil
}

func (s *Store) GetTransaction(ctx context.Context, id string) (*ledger.Transaction, error) {
	var txn ledger.Transaction
	var postedAt string
	var finalized int

	err := s.reader.QueryRowContext(ctx,
		`SELECT id, description, finalized, posted_at FROM transactions WHERE id = ?`, id,
	).Scan(&txn.ID, &txn.Description, &finalized, &postedAt)
	if err == sql.ErrNoRows {
		return nil, ledger.ErrTransactionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	txn.Finalized = finalized == 1
	txn.PostedAt, _ = time.Parse(time.RFC3339Nano, postedAt)

	entries, err := s.getEntriesForTransaction(ctx, id)
	if err != nil {
		return nil, err
	}
	txn.Entries = entries

	return &txn, nil
}

func (s *Store) ListTransactions(ctx context.Context, filter TxnFilter) ([]ledger.Transaction, error) {
	query := `SELECT DISTINCT t.id, t.description, t.finalized, t.posted_at FROM transactions t`
	args := []any{}

	if filter.AccountID != "" {
		query += ` JOIN entries e ON e.transaction_id = t.id WHERE e.account_id = ?`
		args = append(args, filter.AccountID)
	} else {
		query += ` WHERE 1=1`
	}

	query += ` AND t.finalized = 1 ORDER BY t.posted_at DESC`

	if filter.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, filter.Limit)
		if filter.Offset > 0 {
			query += fmt.Sprintf(` OFFSET %d`, filter.Offset)
		}
	}

	rows, err := s.reader.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var txns []ledger.Transaction
	for rows.Next() {
		var txn ledger.Transaction
		var postedAt string
		var finalized int
		if err := rows.Scan(&txn.ID, &txn.Description, &finalized, &postedAt); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		txn.Finalized = finalized == 1
		txn.PostedAt, _ = time.Parse(time.RFC3339Nano, postedAt)

		entries, err := s.getEntriesForTransaction(ctx, txn.ID)
		if err != nil {
			return nil, err
		}
		txn.Entries = entries

		txns = append(txns, txn)
	}
	return txns, rows.Err()
}

func (s *Store) ListEntriesByAccount(ctx context.Context, accountID string, filter EntryFilter) ([]ledger.Entry, error) {
	query := `SELECT e.id, e.transaction_id, e.account_id, e.amount, e.currency, e.created_at
		FROM entries e
		JOIN transactions t ON t.id = e.transaction_id
		WHERE e.account_id = ? AND t.finalized = 1
		ORDER BY e.created_at DESC`

	args := []any{accountID}
	if filter.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, filter.Limit)
		if filter.Offset > 0 {
			query += fmt.Sprintf(` OFFSET %d`, filter.Offset)
		}
	}

	rows, err := s.reader.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

func (s *Store) getEntriesForTransaction(ctx context.Context, txnID string) ([]ledger.Entry, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT id, transaction_id, account_id, amount, currency, created_at FROM entries WHERE transaction_id = ? ORDER BY id`,
		txnID,
	)
	if err != nil {
		return nil, fmt.Errorf("get entries: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

func scanEntries(rows *sql.Rows) ([]ledger.Entry, error) {
	var entries []ledger.Entry
	for rows.Next() {
		var e ledger.Entry
		var createdAt string
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Amount, &e.Currency, &createdAt); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
