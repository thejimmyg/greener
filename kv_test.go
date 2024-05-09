// rm kvstore.db; go run cmd/kvstore/main.go
// Note: If you make create/drop tables outside of this code, it won't notice until you restart. You therefore shouldn't do that.
// Also an application should only have one KV, otherwise the mutex won't behave correclty and you may get database is locked errors due to multiple writers.

package greener_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/thejimmyg/greener"
)

// Useful during inserting test data
func timePtr(t time.Time) *time.Time {
	return &t
}

func TestKVStore(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(func() {
		cancel()
	})

	tempDir, err := ioutil.TempDir("", "db_kvstore_temp")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir) // Clean up the directory when you're done.
	})

	dbPath := filepath.Join(tempDir, "kvstore.db")

	db, err := greener.NewDB(dbPath, 3)
	if err != nil {
		t.Fatalf("Error creating the database connections: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Failed to close database: %v", err)
		}
	})

	tm, err := greener.NewKV(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	tm.StartCleanupRoutine(ctx)

	t.Run("1a. Successful Put and Get", func(t *testing.T) {
		t.Parallel()

		// Example data
		pk := "example/table/1"
		sk := "row1"
		data := greener.JSONValue{
			"field1": "value1",
			"field2": 42.0,
		}
		expires := time.Now().Add(24 * time.Hour).Truncate(time.Second) // Expires in 24 hours

		// 1. Perform a Put operation and retrieve the same data using Get
		if err := tm.Put(ctx, pk, sk, data, &expires); err != nil {
			t.Fatalf("Put operation failed: %v", err)
		}

		retrievedData, retrievedExpires, err := tm.Get(ctx, pk, sk)
		if err != nil {
			t.Fatalf("Get operation failed: %v", err)
		}
		if !reflect.DeepEqual(data, retrievedData) {
			t.Fatalf("The retrieved data doesn't match the inserted data: %v, %v", data, retrievedData)
		}
		if expires != *retrievedExpires {
			t.Fatalf("Expires time after get is wrong: %v %v", expires, *retrievedExpires)
		}
		t.Logf("Put and Get successful\n")
		t.Logf("Retrieved Data: %v\n", retrievedData)
		t.Logf("Retrieved Expire time: %v\n", retrievedExpires)
	})

	t.Run("1b. Get a non-existant key", func(t *testing.T) {
		t.Parallel()
		nonExistentGetPK := "example/nonexistent/1"
		nonExistentGetSK := "nonexistentRow"
		_, _, err := tm.Get(ctx, nonExistentGetPK, nonExistentGetSK)
		if err == nil {
			t.Fatalf("Didn't get the expected error when getting a non-existent key")
		}
	})

	t.Run("2. Create a row that doesn't exist and get the data back successfully", func(t *testing.T) {
		t.Parallel()
		newPK := "example/newtable/1"
		newSK := "newRow1"
		newData := greener.JSONValue{"newField": "newValue", "numberField": 24.0}
		newExpires := time.Now().Add(48 * time.Hour) // Expires in 48 hours
		err = tm.Create(ctx, newPK, newSK, newData, &newExpires)
		if err != nil {
			t.Fatalf("Create operation failed: %v", err)
		} else {
			t.Logf("Row created successfully.\n")
		}

		// Try to get the newly created data back
		retrievedNewData, retrievedNewExpires, err := tm.Get(ctx, newPK, newSK)
		if err != nil {
			t.Fatalf("Failed to retrieve new data: %v", err)
		} else {
			t.Logf("Retrieved New Data: %v\n", retrievedNewData)
			t.Logf("Retrieved New Expires: %v\n", retrievedNewExpires)
		}
	})

	t.Run("3. Put a row that has already expired and check it doesn't come back with a Get", func(t *testing.T) {
		t.Parallel()
		expiredPK := "example/expired/1"
		expiredSK := "expiredRow"
		expiredData := greener.JSONValue{"expiredField": "expiredValue"}
		expiredExpires := time.Now().Add(-1 * time.Hour) // Already expired
		err = tm.Put(ctx, expiredPK, expiredSK, expiredData, &expiredExpires)
		if err != nil {
			t.Fatalf("Put operation with expired data failed: %v", err)
		}

		// Attempt to get the expired data should fail
		_, _, err = tm.Get(ctx, expiredPK, expiredSK)
		if err != nil {
			t.Logf("Correctly did not retrieve expired data: %v\n", err)
		} else {
			t.Fatal("Expired data was unexpectedly retrieved")
		}
	})

	t.Run("4. Create a row that already exists and check the creation fails", func(t *testing.T) {
		t.Parallel()

		pk := "example/table/1"
		sk := "row1"
		data := greener.JSONValue{
			"field1": "value1",
			"field2": 42.0,
		}
		expires := time.Now().Add(24 * time.Hour).Truncate(time.Second) // Expires in 24 hours
		if err := tm.Put(ctx, pk, sk, data, &expires); err != nil {
			t.Fatalf("Put operation failed: %v", err)
		}
		// Trying to create the same row as in the earlier example
		err := tm.Create(ctx, pk, sk, data, &expires)
		if err != nil {
			t.Logf("Succesfully for expected failure on creating an existing row: %v\n", err)
		} else {
			t.Fatal("Unexpected success on creating an existing row")
		}
	})

	t.Run("5. Attempt to delete a row that doesn't exist and check it doesn't give an error", func(t *testing.T) {
		t.Parallel()
		nonExistentPK := "example/nonexistent/1"
		nonExistentSK := "nonexistentRow"
		err = tm.Delete(ctx, nonExistentPK, nonExistentSK)
		if err != nil {
			t.Fatalf("Error when trying to delete a non-existent row: %v", err)
		}
	})

	t.Run("6. Attempt to delete a row from a table that doesn't exist and check it doesn't give an error", func(t *testing.T) {
		t.Parallel()
		nonExistentTablePK := "nonexistent/table/1"
		nonExistentTableSK := "anyRow"
		err = tm.Delete(ctx, nonExistentTablePK, nonExistentTableSK)
		if err != nil {
			t.Fatalf("No error when trying to delete a row from a non-existent table.")
		}
	})

	t.Run("7. Put and then delete a row without an expires and check it doesn't come back in Get", func(t *testing.T) {
		t.Parallel()
		transientPK := "example/transient/1"
		transientSK := "transientRow"
		transientData := greener.JSONValue{"field": "transientValue"}
		// Put the transient row without an expires
		err = tm.Put(ctx, transientPK, transientSK, transientData, nil)
		if err != nil {
			t.Fatalf("Failed to put transient data: %v", err)
		}

		// Now, delete the transient row
		err = tm.Delete(ctx, transientPK, transientSK)
		if err != nil {
			t.Fatalf("Failed to delete transient data: %v", err)
		}

		// Attempt to get the transient data should fail since it's deleted
		_, _, err = tm.Get(ctx, transientPK, transientSK)
		if err != nil {
			t.Logf("Correctly did not retrieve deleted transient data: %v\n", err)
		} else {
			t.Fatal("Unexpectedly retrieved deleted transient data")
		}
	})

	t.Run("8. Testing iteration cases", func(t *testing.T) {
		t.Parallel()
		// Inserting test data
		testData := []struct {
			PK      string
			SK      string
			Data    greener.JSONValue
			Expires *time.Time // Use nil for no expiration, otherwise set a time
		}{
			{"test/table1", "row1", greener.JSONValue{"field": "value1"}, nil},
			{"test/table1", "row2", greener.JSONValue{"field": "value2"}, timePtr(time.Now().Add(48 * time.Hour))},
			{"test/table1", "row2.5-expired", greener.JSONValue{"field": "expired"}, timePtr(time.Now().Add(-48 * time.Hour))},
			{"test/table1", "row3", greener.JSONValue{"field": "value3"}, nil},
			{"test/table1", "row2.5", greener.JSONValue{"field": "value2.5"}, timePtr(time.Now().Add(48 * time.Hour))},
		}

		for _, row := range testData {
			err := tm.Put(ctx, row.PK, row.SK, row.Data, row.Expires)
			if err != nil {
				t.Fatalf("Failed to insert test data for PK: %s, SK: %s, error: %v", row.PK, row.SK, err)
			}
		}
		t.Logf("Test data inserted successfully.\n")

		// Case 8.1 (Basic Iteration Test): Iterating over test/table1 without any sk specified should return row1, row2, row2.5 and row3, excluding the expired row.
		expectedRows := []greener.Row{
			{PK: "test/table1", SK: "row1", Data: greener.JSONValue{"field": "value1"}},
			{PK: "test/table1", SK: "row2", Data: greener.JSONValue{"field": "value2"}},
			{PK: "test/table1", SK: "row2.5", Data: greener.JSONValue{"field": "value2.5"}},
			{PK: "test/table1", SK: "row3", Data: greener.JSONValue{"field": "value3"}},
		}
		rows, _, err := tm.Iterate(ctx, "test/table1", "", 10, false)
		if err != nil {
			t.Fatalf("Iterate failed: %v", err)
		}
		if len(rows) != len(expectedRows) {
			t.Fatalf("Expected %d rows, got %d", len(expectedRows), len(rows))
		}
		for i, row := range rows {
			if row.PK != expectedRows[i].PK || row.SK != expectedRows[i].SK || !reflect.DeepEqual(row.Data, expectedRows[i].Data) {
				t.Fatalf("Row %d did not match expected result. Got %+v, expected %+v", i, row, expectedRows[i])
			}
		}
		t.Logf("Simple insertion worked.\n")

		// Case 2 (Pagination Test): Setting a limit that would cause pagination to occur, ensuring row2.5 appears in the correct page based on the limit.
		// Also testing (After Flag True/False) and  (Empty Results Handling) and also expired row (the last one is missing)

		// First page with limit of 2, expecting the first two rows
		limit := 2
		rows, afterSK, err := tm.Iterate(ctx, "test/table1", "", limit, false)
		if err != nil {
			t.Fatalf("Iterate failed on first page: %v", err)
		}
		if len(rows) != limit {
			t.Fatalf("First page expected %d rows, got %d", limit, len(rows))
		}
		// Check specific row values
		if rows[0].PK != "test/table1" || rows[0].SK != "row1" || rows[1].PK != "test/table1" || rows[1].SK != "row2" {
			t.Fatalf("First page rows do not match expected values: got %+v", rows)
		}
		if afterSK != "row2" {
			t.Fatalf("afterSK after first page should be 'row2', got %s", afterSK)
		}

		t.Logf("First page correct. afterSK: %s\n", afterSK)

		// Second page, expecting to start after 'row2', to get 'row2.5' and 'row3'
		rows, newAfterSK, err := tm.Iterate(ctx, "test/table1", afterSK, limit, true)
		if err != nil {
			t.Fatalf("Iterate failed on second page: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("Second page expected %d rows, got %d", 2, len(rows))
		}
		// Since we expect 'row2.5' and 'row3' to be returned, let's check those
		if rows[0].PK != "test/table1" || rows[0].SK != "row2.5" || rows[1].PK != "test/table1" || rows[1].SK != "row3" {
			t.Fatalf("Second page rows do not match expected values: got %+v", rows)
		}
		if newAfterSK != "row3" {
			t.Fatalf("afterSK after first page should be 'row2', got %s", newAfterSK)
		}
		t.Logf("Second page correct. afterSK: %s\n", newAfterSK)

		// Since we expected the second page to return the last of our rows based on the limit,
		// there shouldn't be more data to fetch. A subsequent call should confirm there are no more rows.
		rows, thirdAfterSK, err := tm.Iterate(ctx, "test/table1", newAfterSK, limit, true)
		if err != nil {
			t.Fatalf("Iterate failed on third page: %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("Third page expected %d rows, got %d", 0, len(rows))
		}
		if thirdAfterSK != newAfterSK {
			t.Fatalf("afterSK after first page should be '%s', got %s", newAfterSK, thirdAfterSK)
		}
		t.Logf("Third page correct. No more rows to fetch.\n")

		// Attempting to iterate over a non-existent table
		nonExistentTablePK1 := "nonexistent/table"
		rows, _, err = tm.Iterate(ctx, nonExistentTablePK1, "", 10, false)
		if err != nil {
			t.Fatalf("Iterate reported an error for non-existent table: %v\n", err)
		} else if len(rows) > 0 {
			t.Fatalf("Iterate should not return rows for a non-existent table, got %d rows", len(rows))
		} else {
			t.Logf("Success, iterate correctly handled non-existent table with no error and no rows.\n")
		}

		// Attempting to iterate over a non-existent pk
		nonExistentPK2 := "test/nonexistent/"
		rows, _, err = tm.Iterate(ctx, nonExistentPK2, "", 10, false)
		if err != nil {
			t.Fatalf("Iterate reported an error for non-existent pk: %v\n", err)
		} else if len(rows) > 0 {
			t.Fatalf("Iterate should not return rows for a non-existent pk, got %d rows", len(rows))
		} else {
			t.Logf("Success. Iterate correctly handled non-existent pk with no error and no rows.\n")
		}
	})

	t.Run("9. Load Testing Puts.", func(t *testing.T) {
		t.Parallel()
		var wg sync.WaitGroup
		for i := 0; i < 2000; i++ {
			wg.Add(1)
			go func(i int, Fatalf func(format string, args ...interface{})) {
				defer wg.Done()
				pk := fmt.Sprintf("concurrent/table/%d", i)
				sk := "row"
				field := fmt.Sprintf("value%d", i)
				data := greener.JSONValue{"field": field}
				// Using nil for Expires to simplify the example, adjust as needed
				err := tm.Put(ctx, pk, sk, data, nil)
				if err != nil {
					Fatalf("Failed to put data for PK: %s, SK: %s, error: %v", pk, sk, err)
				}
				retrievedData, _, err := tm.Get(ctx, pk, sk)
				if err != nil {
					Fatalf("Get operation failed: %v", err)
				}
				if retrievedData["field"] != field {
					Fatalf("Retrieved value didn't match")
				}
			}(i, t.Fatalf)
		}
		t.Logf("All goroutines completed.\n")
		wg.Wait() // Wait for all goroutines to finish
	})

	// Finally check table loading:

	// // Load another KV instance
	// newTM, err := NewKV(db)
	// if err != nil {
	// 	t.Fatalf("Failed to create a new KV: %v", err)
	// }

	// // Manually check for the existence of the test and example tables
	// // The tables names might need to be adjusted based on your schema and naming conventions
	// testTableName, _ := extractTableName("test/table1")
	// exampleTableName, _ := extractTableName("example/table/1")

	// // Check if the test table exists
	// if exists, ok := newTM.tables[testTableName]; !ok || !exists {
	// 	t.Fatalf("Table %s does not exist.", testTableName)
	// } else {
	// 	t.Logf("Table %s found.\n", testTableName)
	// }

	// // Check if the example table exists
	// if exists, ok := newTM.tables[exampleTableName]; !ok || !exists {
	// 	t.Fatalf("Table %s does not exist.", exampleTableName)
	// } else {
	// 	t.Logf("Table %s found.\n", exampleTableName)
	// }

	// // And try concurrency on a different TM. This does work, but might lead to locked errors if done too much.
	// var wg2 sync.WaitGroup
	// for i := 0; i < 2000; i++ {
	// 	wg2.Add(1)
	// 	go func(i int) {
	// 		defer wg2.Done()
	// 		pk := fmt.Sprintf("concurrent/table2/%d", i)
	// 		sk := "row"
	// 		field := fmt.Sprintf("value%d", i)
	// 		data := greener.JSONValue{"field": field}
	// 		// Using nil for Expires to simplify the example, adjust as needed
	// 		err := newTM.Put(ctx, pk, sk, data, nil)
	// 		if err != nil {
	// 			t.Fatalf("Failed to put data for PK: %s, SK: %s, error: %v", pk, sk, err)
	// 		}
	// 		retrievedData, _, err := newTM.Get(ctx, pk, sk)
	// 		if err != nil {
	// 			t.Fatalf("Get operation failed: %v", err)
	// 		}
	// 		if retrievedData["field"] != field {
	// 			t.Fatalf("Retrieved value didn't match")
	// 		}
	// 	}(i)
	// }
	// wg2.Wait() // Wait for all goroutines to finish
	// t.Logf("All goroutines completed.\n")

}
