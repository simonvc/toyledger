package store

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"

	"github.com/simonvc/miniledger/internal/ledger"
	_ "modernc.org/sqlite"
)

type AccountFilter struct {
	Category ledger.Category
	IsSystem *bool
	Limit    int
	Offset   int
}

type TxnFilter struct {
	AccountID string
	Limit     int
	Offset    int
}

type EntryFilter struct {
	Limit  int
	Offset int
}

type Store struct {
	writer *sql.DB
	reader *sql.DB
}

func Open(dbPath string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)", dbPath)

	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open writer: %w", err)
	}
	writer.SetMaxOpenConns(1)

	reader, err := sql.Open("sqlite", dsn)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("open reader: %w", err)
	}
	reader.SetMaxOpenConns(runtime.NumCPU())

	s := &Store{writer: writer, reader: reader}

	if err := s.migrate(context.Background()); err != nil {
		s.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	err1 := s.writer.Close()
	err2 := s.reader.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
