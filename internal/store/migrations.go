package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/simonvc/miniledger/internal/ledger"
)

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create schema version table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		)
	`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	var version int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if version < 1 {
		if err := migrateV1(ctx, tx); err != nil {
			return fmt.Errorf("migration v1: %w", err)
		}
	}

	return tx.Commit()
}

func migrateV1(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		// Accounts table
		`CREATE TABLE IF NOT EXISTS accounts (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			code       INTEGER NOT NULL,
			category   TEXT NOT NULL CHECK (category IN ('assets','liabilities','equity','revenue','expenses')),
			currency   TEXT NOT NULL DEFAULT 'USD',
			is_system  INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_accounts_category ON accounts(category)`,
		`CREATE INDEX IF NOT EXISTS idx_accounts_code ON accounts(code)`,

		// Transactions table
		`CREATE TABLE IF NOT EXISTS transactions (
			id          TEXT PRIMARY KEY,
			description TEXT NOT NULL,
			finalized   INTEGER NOT NULL DEFAULT 0,
			posted_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_posted ON transactions(posted_at)`,

		// Entries table
		`CREATE TABLE IF NOT EXISTS entries (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			transaction_id TEXT NOT NULL REFERENCES transactions(id),
			account_id     TEXT NOT NULL REFERENCES accounts(id),
			amount         INTEGER NOT NULL,
			currency       TEXT NOT NULL,
			created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entries_txn ON entries(transaction_id)`,
		`CREATE INDEX IF NOT EXISTS idx_entries_account ON entries(account_id)`,

		// Trigger: prevent finalizing an unbalanced transaction (per-currency check)
		`CREATE TRIGGER IF NOT EXISTS trg_check_balance
		BEFORE UPDATE OF finalized ON transactions
		WHEN NEW.finalized = 1
		BEGIN
			SELECT CASE
				WHEN EXISTS (
					SELECT currency, SUM(amount) as total
					FROM entries
					WHERE transaction_id = NEW.id
					GROUP BY currency
					HAVING total != 0
				)
				THEN RAISE(ABORT, 'transaction entries do not balance: per-currency sum != 0')
			END;
		END`,

		// Trigger: prevent adding entries to finalized transactions
		`CREATE TRIGGER IF NOT EXISTS trg_immutable_entries_insert
		BEFORE INSERT ON entries
		WHEN (SELECT finalized FROM transactions WHERE id = NEW.transaction_id) = 1
		BEGIN
			SELECT RAISE(ABORT, 'cannot add entries to a finalized transaction');
		END`,

		// Trigger: prevent deleting entries from finalized transactions
		`CREATE TRIGGER IF NOT EXISTS trg_immutable_entries_delete
		BEFORE DELETE ON entries
		WHEN (SELECT finalized FROM transactions WHERE id = OLD.transaction_id) = 1
		BEGIN
			SELECT RAISE(ABORT, 'cannot remove entries from a finalized transaction');
		END`,

		// Trigger: prevent updating entries on finalized transactions
		`CREATE TRIGGER IF NOT EXISTS trg_immutable_entries_update
		BEFORE UPDATE ON entries
		WHEN (SELECT finalized FROM transactions WHERE id = OLD.transaction_id) = 1
		BEGIN
			SELECT RAISE(ABORT, 'cannot modify entries of a finalized transaction');
		END`,

		// Trigger: entry currency must match account currency (except wildcard *)
		`CREATE TRIGGER IF NOT EXISTS trg_entry_currency_match
		BEFORE INSERT ON entries
		WHEN (SELECT currency FROM accounts WHERE id = NEW.account_id) != '*'
			AND NEW.currency != (SELECT currency FROM accounts WHERE id = NEW.account_id)
		BEGIN
			SELECT RAISE(ABORT, 'entry currency does not match account currency');
		END`,

		// Record schema version
		`INSERT INTO schema_version (version) VALUES (1)`,
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:60], err)
		}
	}

	// Seed system accounts
	for _, sa := range ledger.SystemAccounts {
		currency := "USD"
		if sa.ID == "~fx" {
			currency = "*"
		}
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO accounts (id, name, code, category, currency, is_system) VALUES (?, ?, ?, ?, ?, 1)`,
			sa.ID, sa.Name, sa.Code, string(sa.Category), currency,
		)
		if err != nil {
			return fmt.Errorf("seed system account %s: %w", sa.ID, err)
		}
	}

	return nil
}
