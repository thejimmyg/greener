package greener

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"
)

func setupSqlitePragmas(db *sql.DB) error {
	for key, value := range sqlitePragmas {
		if strings.HasPrefix(key, "_") {
			// Strip the leading "_" and set the pragma
			pragma := strings.TrimPrefix(key, "_") + " = " + value
			if _, err := db.Exec("PRAGMA " + pragma); err != nil {
				return err
			}
		}
	}

	return nil
}

func newSQLiteConnectionURL(path string) string {
	connectionUrlParams := make(url.Values)
	for key, value := range sqlitePragmas {
		// For connection string, include keys as they are
		connectionUrlParams.Add(key, value)
	}
	return "file:" + path + "?" + connectionUrlParams.Encode()
}

type writeRequest struct {
	resp chan error
	fn   func(DBHandler) error
}

type DBHandler interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type DBModifier interface {
	Write(func(DBHandler) error) error
}

type DB interface {
	DBHandler
	DBModifier
}

type BatchDB struct {
	DBHandler
	readDB        *sql.DB
	writeDB       *sql.DB
	writeDBLock   sync.Mutex
	writeRequests chan writeRequest
	flushTimeout  time.Duration
}

func NewBatchDB(path string, flushTimeout time.Duration) (*BatchDB, error) {

	connectionURL := newSQLiteConnectionURL(path)
	// fmt.Println(connectionURL)

	writeDB, err := sql.Open(SqlDriver, connectionURL+"&mode=rwc")
	if err != nil {
		return nil, err
	}
	writeDB.SetMaxOpenConns(1)
	if err = setupSqlitePragmas(writeDB); err != nil {
		return nil, err
	}

	// Put the read connection into literally read only mode.
	ReadDB, err := sql.Open(SqlDriver, connectionURL+"&mode=ro")
	if err != nil {
		return nil, err
	}
	maxConns := 4
	if n := runtime.NumCPU(); n > maxConns {
		maxConns = n
	}
	ReadDB.SetMaxOpenConns(maxConns)
	if err = setupSqlitePragmas(ReadDB); err != nil {
		return nil, err
	}

	db := &BatchDB{
		DBHandler:     ReadDB,
		readDB:        ReadDB,
		writeDB:       writeDB,
		writeRequests: make(chan writeRequest),
		flushTimeout:  flushTimeout,
	}

	go db.batchProcessor()
	return db, nil
}

func (db *BatchDB) batchProcessor() {
	var requests []writeRequest
	var currentTx *sql.Tx
	timer := time.NewTicker(db.flushTimeout * time.Millisecond)

	for {
		select {
		case req := <-db.writeRequests:
			if len(requests) == 0 {
				var err error
				currentTx, err = db.writeDB.Begin()
				if err != nil {
					req.resp <- err
					continue
				}
			}
			err := req.fn(currentTx)
			if err != nil {
				// fmt.Printf("Rolling back: %v\n", err)
				rollbackErr := currentTx.Rollback()
				if rollbackErr != nil {
					fmt.Printf("Error rolling back: %v. Original error: %v\n", rollbackErr, err)
				}
				req.resp <- err
				for _, r := range requests {
					r.resp <- fmt.Errorf("Transaction aborted")
				}
				requests = requests[:0]
				continue
			}
			requests = append(requests, req)
		case <-timer.C:
			if len(requests) > 0 {
				// fmt.Printf("Committing\n")
				commitErr := currentTx.Commit()
				currentTx = nil
				for _, req := range requests {
					req.resp <- commitErr
				}
				requests = requests[:0]
			}
		}
	}
}

func (db *BatchDB) Write(fn func(DBHandler) error) error {
	respChan := make(chan error)
	req := writeRequest{
		fn:   fn,
		resp: respChan,
	}
	db.writeRequests <- req
	err := <-respChan
	return err
}

func (db *BatchDB) Close() error {
	rerr := db.readDB.Close()
	db.writeDBLock.Lock()
	defer db.writeDBLock.Unlock()
	werr := db.writeDB.Close()
	if rerr != nil || werr != nil {
		return fmt.Errorf("Error closing connections. Write DB Err: %v. Read DB err: %v.\n", werr, rerr)
	}
	return nil
}
