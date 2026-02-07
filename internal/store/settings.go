package store

import (
	"context"
	"fmt"

	"github.com/simonvc/miniledger/internal/ledger"
)

func (s *Store) ListAllSettings(ctx context.Context) ([]ledger.CoASetting, error) {
	rows, err := s.reader.QueryContext(ctx, `SELECT code, setting, value FROM coa_settings ORDER BY code, setting`)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()

	var settings []ledger.CoASetting
	for rows.Next() {
		var cs ledger.CoASetting
		if err := rows.Scan(&cs.Code, &cs.Setting, &cs.Value); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		settings = append(settings, cs)
	}
	return settings, rows.Err()
}

func (s *Store) GetCodeSettings(ctx context.Context, code int) (ledger.CodeSettings, error) {
	cs := ledger.DefaultCodeSettings(code)

	rows, err := s.reader.QueryContext(ctx,
		`SELECT setting, value FROM coa_settings WHERE code = ?`, code)
	if err != nil {
		return cs, fmt.Errorf("get code settings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name ledger.SettingName
		var value string
		if err := rows.Scan(&name, &value); err != nil {
			return cs, fmt.Errorf("scan code setting: %w", err)
		}
		switch name {
		case ledger.SettingBlockInverted:
			cs.BlockInverted = value == "1"
		case ledger.SettingEntryDirection:
			cs.EntryDirection = ledger.EntryDirection(value)
		}
	}
	return cs, rows.Err()
}

func (s *Store) UpsertSetting(ctx context.Context, setting ledger.CoASetting) error {
	_, err := s.writer.ExecContext(ctx,
		`INSERT INTO coa_settings (code, setting, value) VALUES (?, ?, ?)
		 ON CONFLICT(code, setting) DO UPDATE SET value = excluded.value`,
		setting.Code, setting.Setting, setting.Value,
	)
	if err != nil {
		return fmt.Errorf("upsert setting: %w", err)
	}
	return nil
}

func (s *Store) DeleteSetting(ctx context.Context, code int, name ledger.SettingName) error {
	_, err := s.writer.ExecContext(ctx,
		`DELETE FROM coa_settings WHERE code = ? AND setting = ?`, code, name)
	if err != nil {
		return fmt.Errorf("delete setting: %w", err)
	}
	return nil
}
