// rm search_engine.db ; go run -tags "sqlite_fts5" main.go

package greener

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strings"
)

type FTS struct {
	db DB
}

type Facet struct {
	Name  string
	Value string
}

type FacetValueCount struct {
	Value string
	Count int
}

type FacetCount struct {
	Name   string
	Values []FacetValueCount
}

func NewFTS(ctx context.Context, db DB) (*FTS, error) {
	// Ensure the FTS table and facet tables exist
	// _, err = d.ExecContext(ctx, "INSERT INTO document_facets (document_id, facet_id) VALUES (?, ?)", docid, facetID)
	// search_test.go:64: Error adding facets: Could not insert document_facet: SQL logic error: foreign key mismatch - "document_facets" referencing "documents" (1)

	queries := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS documents USING fts5(content, docid UNINDEXED);`,
		`CREATE TABLE IF NOT EXISTS facets (id INTEGER PRIMARY KEY, name TEXT, value TEXT, UNIQUE(name, value));`,
		// `CREATE TABLE IF NOT EXISTS document_facets (document_id TEXT, facet_id INTEGER, FOREIGN KEY(document_id) REFERENCES documents(docid), FOREIGN KEY(facet_id) REFERENCES facets(id));`,
		`CREATE TABLE IF NOT EXISTS document_facets (document_id TEXT, facet_id INTEGER, FOREIGN KEY(facet_id) REFERENCES facets(id));`,
	}
	for _, query := range queries {
		err := db.Write(func(d WriteDBHandler) error {
			if _, err := d.ExecContext(ctx, query); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return &FTS{db: db}, nil
}

func (se *FTS) Put(ctx context.Context, docid string, reader io.Reader) error {
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}

	err = se.db.Write(func(d WriteDBHandler) error {
		// I think we need to do the two operations separately because of a limitation in FT5 virtual tables, but should check this again.

		// Attempt to update the document first.
		result, err := d.ExecContext(ctx, "UPDATE documents SET content = ? WHERE docid = ?", string(content), docid)
		if err != nil {
			return err
		}
		// Check if the update operation affected any rows.
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}

		// If no rows were affected by the update, the document does not exist and needs to be inserted.
		if rowsAffected == 0 {
			_, err = d.ExecContext(ctx, "INSERT INTO documents(docid, content) VALUES(?, ?)", docid, string(content))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (se *FTS) Delete(ctx context.Context, docid string) error {
	err := se.db.Write(func(d WriteDBHandler) error {
		_, err := d.ExecContext(ctx, "DELETE FROM documents WHERE docid = ?", docid)
		return err
	})
	if err != nil {
		return err
	}
	return nil
}

func (se *FTS) Get(ctx context.Context, docid string) (io.Reader, error) {
	var content string
	row := se.db.QueryRowContext(ctx, "SELECT content FROM documents WHERE docid = ?", docid)
	if err := row.Scan(&content); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("document not found")
		}
		return nil, err
	}
	return strings.NewReader(content), nil
}

func (se *FTS) Search(ctx context.Context, query string) ([]map[string]string, error) {
	rows, err := se.db.QueryContext(ctx, "SELECT docid, snippet(documents, 0, '<b>', '</b>', '...', 64) FROM documents WHERE documents MATCH ? ORDER BY rank", query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]string
	for rows.Next() {
		var docid, snippet string
		if err := rows.Scan(&docid, &snippet); err != nil {
			return nil, err
		}
		results = append(results, map[string]string{"docid": docid, "content": snippet})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (se *FTS) AddFacets(ctx context.Context, docid string, facets []Facet) error {
	for _, facet := range facets {
		err := se.db.Write(func(d WriteDBHandler) error {
			result, err := d.ExecContext(ctx, "INSERT INTO facets (name, value) VALUES (?, ?) ON CONFLICT(name, value) DO NOTHING", facet.Name, facet.Value)
			if err != nil {
				return fmt.Errorf("Could not insert facet: %v\n", err)
			}

			facetID, err := result.LastInsertId()
			if err != nil || facetID == 0 {
				err = d.QueryRowContext(ctx, "SELECT id FROM facets WHERE name = ? AND value = ?", facet.Name, facet.Value).Scan(&facetID)
				if err != nil {
					return fmt.Errorf("Could not get facet ID: %v\n", err)
				}
			}

			_, err = d.ExecContext(ctx, "INSERT INTO document_facets (document_id, facet_id) VALUES (?, ?)", docid, facetID)
			if err != nil {
				return fmt.Errorf("Could not insert document_facet: %v\n", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (se *FTS) GetFacetCounts(ctx context.Context, docIDs []string) ([]FacetCount, error) {
	if len(docIDs) == 0 {
		return []FacetCount{}, nil
	}

	inParams := strings.Repeat("?,", len(docIDs)-1) + "?"
	query := fmt.Sprintf("SELECT f.name, f.value, COUNT(*) as count FROM document_facets df JOIN facets f ON df.facet_id = f.id WHERE df.document_id IN (%s) GROUP BY f.name, f.value ORDER BY f.name, count DESC", inParams)

	rows, err := se.db.QueryContext(ctx, query, stringsToInterfaces(docIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	facetCountsMap := make(map[string][]FacetValueCount)
	for rows.Next() {
		var name, value string
		var count int
		if err := rows.Scan(&name, &value, &count); err != nil {
			return nil, err
		}

		facetCountsMap[name] = append(facetCountsMap[name], FacetValueCount{Value: value, Count: count})
	}

	var facetCounts []FacetCount
	for name, values := range facetCountsMap {
		facetCounts = append(facetCounts, FacetCount{Name: name, Values: values})
	}

	return facetCounts, nil
}

func SortFacetsByTotalDocCount(facets []FacetCount) {
	sort.Slice(facets, func(i, j int) bool {
		iTotal, jTotal := 0, 0
		for _, v := range facets[i].Values {
			iTotal += v.Count
		}
		for _, v := range facets[j].Values {
			jTotal += v.Count
		}
		return iTotal > jTotal
	})
}

func OrderFacetsByNames(facets []FacetCount, order []string) []FacetCount {
	orderMap := make(map[string]int)
	for i, name := range order {
		orderMap[name] = i
	}

	sortedFacets := make([]FacetCount, len(facets))
	copy(sortedFacets, facets)

	sort.SliceStable(sortedFacets, func(i, j int) bool {
		iOrder, iFound := orderMap[sortedFacets[i].Name]
		jOrder, jFound := orderMap[sortedFacets[j].Name]

		if iFound && jFound {
			return iOrder < jOrder
		}
		if iFound {
			return true
		}
		if jFound {
			return false
		}
		return sortedFacets[i].Name < sortedFacets[j].Name
	})

	return sortedFacets
}

func stringsToInterfaces(strings []string) []interface{} {
	interfaces := make([]interface{}, len(strings))
	for i, s := range strings {
		interfaces[i] = s
	}
	return interfaces
}

// Extracts docIDs from search results
func getDocIDsFromSearchResults(results []map[string]string) []string {
	var docIDs []string
	for _, result := range results {
		docIDs = append(docIDs, result["docid"])
	}
	return docIDs
}
