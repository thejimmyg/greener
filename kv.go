// rm kvstore.db; go run cmd/kvstore/main.go
// Note: If you make create/drop tables outside of this code, it won't notice until you restart. You therefore shouldn't do that.
// Also an application should only have one KV, otherwise the mutex won't behave correclty and you may get database is locked errors due to multiple writers.

package greener

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// JSONValue is a map that can only contain number or string values but will be stored encoded in JSON.
type JSONValue map[string]interface{}

// MarshalJSON ensures the values are either float64 or string.
func (j JSONValue) MarshalJSON() ([]byte, error) {
	temp := make(map[string]interface{})
	for k, v := range j {
		switch v.(type) {
		case float64, string:
			temp[k] = v
		default:
			return nil, fmt.Errorf("JSONValue must be a map of strings to either float64 or string values")
		}
	}
	return json.Marshal(temp)
}

// UnmarshalJSON ensures the values are either float64 or string.
func (j *JSONValue) UnmarshalJSON(data []byte) error {
	temp := make(map[string]interface{})
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	for _, v := range temp {
		switch v.(type) {
		case float64, string:
			continue
		default:
			return fmt.Errorf("JSONValue must be a map of strings to either float64 or string values")
		}
	}
	*j = temp
	return nil
}

// Row represents a single row returned by the Iterate method.
type Row struct {
	PK      string
	SK      string
	Expires *time.Time // This is a pointer so that it can be nil, representing a NULL value in SQL
	Data    JSONValue
}

// KvStore is the interface defining the key value store operations.
type KvStore interface {
	Create(pk string, sk string, data JSONValue, expires *time.Time) error
	Put(pk string, sk string, data JSONValue, expires *time.Time) error
	Delete(pk, sk string) error
	Iterate(pk, skStart string, limit int, after string) ([]Row, string, error)
	Get(pk, sk string) (JSONValue, *time.Time, error)
}

// KV keeps track of KVstore tables and manages the database connection.
type KV struct {
	db DB
}

// NewKV initializes and returns a new KV.
func NewKV(ctx context.Context, db DB) (*KV, error) {
	tm := &KV{
		db: db,
	}

	err := tm.db.Write(func(writeDB DBHandler) error {
		if err := tm.ensureTableExists(ctx, "kv", writeDB); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tm, nil
}

// There would be a race condition here, but since Put/Create/Delete all obtain an exclusive mutex, and since they are the only functions that call it, we're OK.
func (tm *KV) ensureTableExists(ctx context.Context, tableName string, writeDB DBHandler) error {
	createTableSQL := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s (
            pk TEXT NOT NULL,
            sk TEXT NOT NULL,
            data JSON NOT NULL,
            expires INTEGER,
            PRIMARY KEY (pk, sk)
        );`, tableName)

	// fmt.Printf("Creating table: %s\n", tableName)
	_, err := writeDB.ExecContext(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}
	return nil
}

// StartCleanupRoutine runs a goroutine that periodically deletes expired rows from KVstore tables.
func (tm *KV) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				now := time.Now().Unix()
				tableName := "kv"
				err := tm.db.Write(func(writeDB DBHandler) error {

					_, err := writeDB.ExecContext(ctx, "DELETE FROM "+tableName+" WHERE expires IS NOT NULL AND expires < ?", now)
					if err != nil {
						log.Printf("Error cleaning up expired rows in table %s: %v", tableName, err)
					}
					return nil
				})
				if err != nil {
					log.Printf("Error cleaning up expired rows in table %s: %v", tableName, err)
				}
			}
		}
	}()
}

func (tm *KV) putOrCreate(ctx context.Context, pk string, sk string, data JSONValue, expires *time.Time, allowUpdate bool) error {

	tableName := "kv"
	changed := true
	err := tm.db.Write(func(writeDB DBHandler) error {

		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("Error encoding data to JSON: %w", err)
		}
		// fmt.Printf("Connection: %v\n", tm.db)
		var expiresUnix *int64
		if expires != nil {
			unix := expires.Unix()
			expiresUnix = &unix
		}

		if allowUpdate {
			upsertSQL := fmt.Sprintf(`
        	    INSERT INTO %s (pk, sk, data, expires) VALUES (?, ?, ?, ?)
        	    ON CONFLICT(pk, sk) DO UPDATE SET data=excluded.data, expires=excluded.expires;
        	`, tableName)
			_, err = writeDB.ExecContext(ctx, upsertSQL, pk, sk, jsonData, expiresUnix)
			if err != nil {
				return fmt.Errorf("Failed to upsert row in table %s: %w", tableName, err)
			}
			return nil
		} else {

			insertSQL := fmt.Sprintf(`
        	    INSERT INTO %s (pk, sk, data, expires) VALUES (?, ?, ?, ?)
        	    ON CONFLICT(pk, sk) DO NOTHING;
        	`, tableName)
			result, err := writeDB.ExecContext(ctx, insertSQL, pk, sk, jsonData, expiresUnix)
			if err != nil {
				return fmt.Errorf("failed to insert row in table %s: %w", tableName, err)
			}
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("Error checking rows affected for table %s: %w", tableName, err)
			}
			if rowsAffected == 0 {
				// Row with pk and sk already exists and we have simply ignored it.
				// Crucially, this is not an error
				changed = false
			}
			return nil
		}
	})
	if err != nil {
		return fmt.Errorf("Failed to put row in table %s: %w", tableName, err)
	}
	if !allowUpdate && !changed {
		// The create failed
		return fmt.Errorf("Row with pk %s and sk %s already exists", pk, sk)
	}

	return nil
}

// Put inserts or updates a row with the given pk, sk, data, and expires.
func (tm *KV) Put(ctx context.Context, pk string, sk string, data JSONValue, expires *time.Time) error {
	return tm.putOrCreate(ctx, pk, sk, data, expires, true) // true allows updates
}

// Create inserts a row with the given pk, sk, data, and expires, but fails if the row already exists.
func (tm *KV) Create(ctx context.Context, pk string, sk string, data JSONValue, expires *time.Time) error {
	return tm.putOrCreate(ctx, pk, sk, data, expires, false) // false disallows updates, failing on conflict
}

// Get retrieves a row with the given pk and sk. It returns the data and expires if the row exists and is not expired.
func (tm *KV) Get(ctx context.Context, pk string, sk string) (JSONValue, *time.Time, error) {
	tableName := "kv"

	querySQL := fmt.Sprintf(`
        SELECT data, expires FROM %s WHERE pk = ? AND sk = ? AND (expires IS NULL OR expires > ?);
    `, tableName)

	var jsonData string
	var expiresUnix sql.NullInt64
	err := tm.db.QueryRowContext(ctx, querySQL, pk, sk, time.Now().Unix()).Scan(&jsonData, &expiresUnix)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("no matching row found")
		}
		return nil, nil, fmt.Errorf("error querying for row: %w", err)
	}

	var data JSONValue
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, nil, fmt.Errorf("error decoding data from JSON: %w", err)
	}

	var expires *time.Time
	if expiresUnix.Valid {
		t := time.Unix(expiresUnix.Int64, 0)
		expires = &t
	}

	return data, expires, nil
}

// Delete removes a row with the given pk and sk from the table.
func (tm *KV) Delete(ctx context.Context, pk string, sk string) error {
	tableName := "kv"

	// Prepare the DELETE statement
	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE pk = ? AND sk = ?", tableName)

	err := tm.db.Write(func(writeDB DBHandler) error {
		_, err := writeDB.ExecContext(ctx, deleteSQL, pk, sk)
		if err != nil {
			return fmt.Errorf("failed to delete row from table %s with pk %s and sk %s: %w", tableName, pk, sk, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to delete row from table %s with pk %s and sk %s: %w", tableName, pk, sk, err)
	}
	return err
}

// Iterate over rows in a table based on primary key and sort key.
// If 'after' is true, search for rows with sort keys strictly greater than 'sk'.
// Otherwise, include rows with sort keys greater than or equal to 'sk'.
func (tm *KV) Iterate(ctx context.Context, pk, sk string, limit int, after bool) ([]Row, string, error) {
	tableName := "kv"

	var rows []Row
	var querySQL string
	var args []interface{}

	skCondition := ">="
	if after {
		skCondition = ">"
	}

	if sk != "" {
		querySQL = fmt.Sprintf(`
            SELECT pk, sk, data, expires FROM %s
            WHERE pk = ? AND sk %s ? AND (expires IS NULL OR expires > ?)
            ORDER BY sk ASC
            LIMIT ?;`, tableName, skCondition)
		args = []interface{}{pk, sk, time.Now().Unix(), limit}
	} else {
		querySQL = fmt.Sprintf(`
            SELECT pk, sk, data, expires FROM %s
            WHERE pk = ? AND (expires IS NULL OR expires > ?)
            ORDER BY sk ASC
            LIMIT ?;`, tableName)
		args = []interface{}{pk, time.Now().Unix(), limit}
	}

	sqlRows, err := tm.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, "", fmt.Errorf("error executing iterate query: %w", err)
	}
	defer sqlRows.Close()

	for sqlRows.Next() {
		var r Row
		var expiresUnix sql.NullInt64
		var jsonData string
		if err := sqlRows.Scan(&r.PK, &r.SK, &jsonData, &expiresUnix); err != nil {
			return nil, "", fmt.Errorf("error scanning row: %w", err)
		}

		if err := json.Unmarshal([]byte(jsonData), &r.Data); err != nil {
			return nil, "", fmt.Errorf("error unmarshaling JSON data: %w", err)
		}

		if expiresUnix.Valid {
			expires := time.Unix(expiresUnix.Int64, 0)
			r.Expires = &expires
		}

		rows = append(rows, r)
	}

	// Generate a new 'after' token for pagination, based on the last 'sk' value seen
	newAfter := sk
	if len(rows) > 0 {
		newAfter = rows[len(rows)-1].SK
	}

	return rows, newAfter, nil
}
