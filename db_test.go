package greener_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/thejimmyg/greener"
)

func TestBatchDB(t *testing.T) {
	t.Parallel()

	const count = 10_000
	const maxConcurrency = 1000
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(func() {
		cancel()
	})

	tempDir, err := ioutil.TempDir("", "db_temp")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir) // Clean up the directory when you're done.
	})

	dbPath := filepath.Join(tempDir, "test.db")
	t.Logf("Batch DB path: %s", dbPath)

	db, err := greener.NewBatchDB(dbPath, 3)
	if err != nil {
		t.Fatalf("Error creating the database connections: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Failed to close database: %v", err)
		}
	})

	err = db.Write(func(writeDB greener.DBHandler) error {
		createTableSQL := `CREATE TABLE IF NOT EXISTS greetings (id INTEGER PRIMARY KEY, greeting TEXT)`
		if _, err := writeDB.ExecContext(ctx, createTableSQL); err != nil {
			return err
		}

		insertRowsSQL := `INSERT INTO greetings (greeting) VALUES (?), (?)`
		if _, err := writeDB.ExecContext(ctx, insertRowsSQL, "Hello, World!", "Hi, Universe!"); err != nil {
			return err
		}

		insertAndReturnSQL := `INSERT INTO greetings (greeting) VALUES (?) RETURNING id, greeting`
		rows, err := writeDB.QueryContext(ctx, insertAndReturnSQL, "Greetings, Multiverse!")
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var id int
			var greeting string
			if err := rows.Scan(&id, &greeting); err != nil {
				return err
			}
			t.Logf("Inserted greeting with id %d: %s", id, greeting)
		}
		return rows.Err()
	})

	if err != nil {
		t.Fatal(err)
	}

	// Simplified read operations using ReadDB directly
	selectSQL := `SELECT greeting FROM greetings`
	rows, err := db.QueryContext(ctx, selectSQL)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var greeting string
		if err := rows.Scan(&greeting); err != nil {
			t.Fatal(err)
		}
		// t.Log(greeting + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	// go monitorMemUsage()
	t.Logf("Starting batch inserting %d greetings.\n", count)

	start := time.Now()
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrency) // Buffer size defines max concurrency

	for i := 0; i < count; i++ {
		wg.Add(1)
		semaphore <- struct{}{} // Block if semaphore is full (maxConcurrency goroutines running)

		go func(i int, Fatalf func(format string, args ...interface{})) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore once done, allowing another goroutine to proceed

			err := db.Write(func(writeDB greener.DBHandler) error {
				greeting := fmt.Sprintf("Hello, World #%d!", i)
				insertGreetingSQL := `INSERT INTO greetings (greeting) VALUES (?)`
				_, err := writeDB.ExecContext(ctx, insertGreetingSQL, greeting)
				return err
			})
			if err != nil {
				Fatalf("Failed to insert greeting: %v", err)
			}
		}(i, t.Fatalf)
	}

	wg.Wait()        // Wait for all goroutines to finish
	close(semaphore) // Close the semaphore channel

	diff := time.Now().Sub(start)
	seconds := float64(diff) / float64(time.Second)
	t.Logf("Completed batch inserting %d greetings in %s, %f greetings per second\n", count, diff, float64(count)/seconds)

	greeting := fmt.Sprintf("Hello, World #err1!")
	t.Logf("Trying out an error\n")
	err = db.Write(func(writeDB greener.DBHandler) error {
		insertGreetingSQL := `INSERT INTO not_a_real_greetings_table (greeting) VALUES (?)`
		_, err := writeDB.ExecContext(ctx, insertGreetingSQL, greeting)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Logf("Failed to insert greeting: %v", err)
	}

	t.Logf("Trying again\n")
	err = db.Write(func(writeDB greener.DBHandler) error {
		insertGreetingSQL := `INSERT INTO greetings (greeting) VALUES (?)`
		_, err := writeDB.ExecContext(ctx, insertGreetingSQL, greeting)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Logf("Failed to insert greeting: %v", err)
	}
	t.Logf("Reached the end\n")

}
