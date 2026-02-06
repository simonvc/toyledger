package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/simonvc/miniledger/internal/ledger"
)

func (s *Store) CreateAccount(ctx context.Context, acct *ledger.Account) error {
	if err := acct.Validate(); err != nil {
		return err
	}

	_, err := s.writer.ExecContext(ctx,
		`INSERT INTO accounts (id, name, code, category, currency, is_system) VALUES (?, ?, ?, ?, ?, ?)`,
		acct.ID, acct.Name, acct.Code, string(acct.Category), acct.Currency, boolToInt(acct.IsSystem),
	)
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

func (s *Store) GetAccount(ctx context.Context, id string) (*ledger.Account, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT id, name, code, category, currency, is_system, created_at FROM accounts WHERE id = ?`, id)
	return scanAccount(row)
}

func (s *Store) ListAccounts(ctx context.Context, filter AccountFilter) ([]ledger.Account, error) {
	query := `SELECT id, name, code, category, currency, is_system, created_at FROM accounts WHERE 1=1`
	args := []any{}

	if filter.Category != "" {
		query += ` AND category = ?`
		args = append(args, string(filter.Category))
	}
	if filter.IsSystem != nil {
		query += ` AND is_system = ?`
		args = append(args, boolToInt(*filter.IsSystem))
	}

	query += ` ORDER BY code`

	if filter.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, filter.Limit)
		if filter.Offset > 0 {
			query += fmt.Sprintf(` OFFSET %d`, filter.Offset)
		}
	}

	rows, err := s.reader.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []ledger.Account
	for rows.Next() {
		acct, err := scanAccountRow(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *acct)
	}
	return accounts, rows.Err()
}

func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	// Check account exists
	_, err := s.GetAccount(ctx, id)
	if err != nil {
		return err
	}

	// Refuse if account has any entries
	var count int
	err = s.reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entries WHERE account_id = ?`, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("check entries: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete account %s: has %d entries", id, count)
	}

	_, err = s.writer.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}

func scanAccount(row *sql.Row) (*ledger.Account, error) {
	var acct ledger.Account
	var isSystem int
	var createdAt string
	err := row.Scan(&acct.ID, &acct.Name, &acct.Code, &acct.Category, &acct.Currency, &isSystem, &createdAt)
	if err == sql.ErrNoRows {
		return nil, ledger.ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan account: %w", err)
	}
	acct.IsSystem = isSystem == 1
	acct.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return &acct, nil
}

func scanAccountRow(rows *sql.Rows) (*ledger.Account, error) {
	var acct ledger.Account
	var isSystem int
	var createdAt string
	err := rows.Scan(&acct.ID, &acct.Name, &acct.Code, &acct.Category, &acct.Currency, &isSystem, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("scan account row: %w", err)
	}
	acct.IsSystem = isSystem == 1
	acct.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return &acct, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
