package greener

import (
	"context"
	"database/sql"
	"fmt"
)

type txWrapper struct {
	tx  *sql.Tx
	err error
}

func (t *txWrapper) Abort(err error) {
	if t.err != nil {
		panic("Abort called again when there was already an error")
	}
	t.err = err
	rollbackErr := t.tx.Rollback()
	if rollbackErr != nil {
		fmt.Printf("Error rolling back: %v. Original error: %v\n", rollbackErr, err)
	}
}

func (t *txWrapper) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if t.err != nil {
		return nil, fmt.Errorf("this transaction is already aborted")
	}
	result, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		t.Abort(err)
	}
	return result, err
}

func (t *txWrapper) QueryContext(ctx context.Context, query string, args ...interface{}) (*rowsWrapper, error) {
	if t.err != nil {
		return nil, fmt.Errorf("this transaction is already aborted")
	}
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		t.Abort(err)
		return nil, err
	}
	return &rowsWrapper{rows: rows, txWrapper: t}, nil
}

func (t *txWrapper) QueryRowContext(ctx context.Context, query string, args ...interface{}) *rowWrapper {
	if t.err != nil {
		return &rowWrapper{row: nil, txWrapper: t}
	}
	return &rowWrapper{row: t.tx.QueryRowContext(ctx, query, args...), txWrapper: t}
}

type rowWrapper struct {
	row       *sql.Row
	txWrapper *txWrapper
}

func (r *rowWrapper) Scan(dest ...interface{}) error {
	if r.txWrapper.err != nil {
		return fmt.Errorf("this transaction is already aborted")
	}
	err := r.row.Scan(dest...)
	if err != nil {
		r.txWrapper.Abort(err)
	}
	return err
}

type rowsWrapper struct {
	rows      *sql.Rows
	txWrapper *txWrapper
}

func (r *rowsWrapper) Scan(dest ...interface{}) error {
	if r.txWrapper.err != nil {
		return fmt.Errorf("this transaction is already aborted")
	}
	err := r.rows.Scan(dest...)
	if err != nil {
		r.txWrapper.Abort(err)
	}
	return err
}

func (r *rowsWrapper) Next() bool {
	if r.txWrapper.err != nil {
		return false
	}
	return r.rows.Next()
}

func (r *rowsWrapper) Close() error {
	if r.txWrapper.err != nil {
		return fmt.Errorf("this transaction is already aborted")
	}
	return r.rows.Close()
}

func (r *rowsWrapper) Err() error {
	if r.txWrapper.err != nil {
		return fmt.Errorf("this transaction is already aborted")
	}
	return r.rows.Err()
}
