// rm search_engine.db ; go run -tags "sqlite_fts5" main.go

package greener

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSearch(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(func() {
		cancel()
	})

	tempDir, err := ioutil.TempDir("", "db_search_temp")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir) // Clean up the directory when you're done.
	})

	searchDbPath := filepath.Join(tempDir, "search.db")
	t.Logf("Search DB path: %s", searchDbPath)

	db, err := NewDB(searchDbPath, 3)
	if err != nil {
		t.Fatalf("Error creating the database connections: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Failed to close database: %v", err)
		}
	})

	se, err := NewSearchEngine(ctx, db)
	if err != nil {
		t.Error(err)
	}

	docID := "1"
	content := "This <em>is</em> a simple search engine example."

	// Demonstrate Put
	if err := se.Put(ctx, docID, strings.NewReader(content)); err != nil {
		t.Error(err)
	}
	t.Logf("Document inserted\n")

	// Demonstrate Get
	reader, err := se.Get(ctx, docID)
	if err != nil {
		t.Fatal(err)
	}

	retrievedContent, _ := ioutil.ReadAll(reader)
	t.Logf("Retrieved content: %s\n", string(retrievedContent))

	// Add facets to the document
	facets := []Facet{
		{"Year", "2021"},
		{"Author", "John Doe"},
	}
	if err := se.AddFacets(ctx, docID, facets); err != nil {
		t.Errorf("Error adding facets: %v\n", err)
	}
	t.Logf("Facets added\n")

	// Demonstrate Get
	reader, err = se.Get(ctx, docID)
	if err != nil {
		t.Error(err)
	}

	retrievedContent, _ = ioutil.ReadAll(reader)
	t.Logf("Retrieved content: %s\n", string(retrievedContent))

	// Search and retrieve docIDs from search results
	query := "search engine"
	results, err := se.Search(ctx, query)
	if err != nil {
		t.Error(err)
	}

	t.Logf("Search results:\n")
	for _, result := range results {
		t.Logf("DocID: %s, Content: %s\n", result["docid"], result["content"])
	}

	// Extract docIDs from search results
	docIDs := getDocIDsFromSearchResults(results)

	// Fetch facet counts for these docIDs
	facetCounts, err := se.GetFacetCounts(ctx, docIDs)
	if err != nil {
		t.Errorf("Couldn't get facet counts: %v\n", err)
	}

	// Sort facet counts by total document count within each facet
	SortFacetsByTotalDocCount(facetCounts)

	// Print out sorted facet counts
	t.Logf("Sorted Facet Counts:\n")
	for _, facetCount := range facetCounts {
		t.Logf("Facet Name: %s\n", facetCount.Name)
		for _, valCount := range facetCount.Values {
			t.Logf("  Value: %s, Count: %d\n", valCount.Value, valCount.Count)
		}
	}

	// Demonstrate Delete
	if err := se.Delete(ctx, docID); err != nil {
		t.Error(err)
	}
	t.Logf("Document deleted\n")

	// Try to retrieve again
	_, err = se.Get(ctx, docID)
	if err != nil {
		t.Logf("Successfully got error retrieving deleted document: %v\n", err)
	} else {
		t.Errorf("Expected an error retrieving a deleted document but didn't get one.\n")
	}
}
