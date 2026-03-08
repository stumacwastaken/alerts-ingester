package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Use the store like a repository to avoid having the sql.db reference
// everywhere.
type Store struct {
	*Queries
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	queries := New(db)
	return &Store{
		Queries: queries,
		db:      db,
	}
}

var ErrTXRollback = errors.New("failed to rollback transaction")
var ErrTransactionFailed = errors.New("failed to apply query")
var ErrTransactionCommitFailed = errors.New("failed to commit transaction")

func (s *Store) ExecInTx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	queries := s.WithTx(tx)
	err = fn(queries)
	if err != nil {
		rbErr := tx.Rollback()
		if rbErr != nil {
			return fmt.Errorf("%w details: %w", ErrTXRollback, rbErr)
		}
		return fmt.Errorf("%w details: %w", ErrTransactionFailed, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w details: %w", ErrTransactionCommitFailed, err)
	}
	return nil

}
