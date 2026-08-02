package graph

import (
	"fmt"
	"strconv"

	"github.com/palemoky/chinese-poetry-api/internal/database"
	"github.com/palemoky/chinese-poetry-api/internal/helpers"
)

// Pagination holds parsed pagination parameters
type Pagination struct {
	Page     int
	PageSize int
	Offset   int
}

// Pagination defaults and bounds, kept in step with the REST handler's.
const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// parsePagination extracts and validates pagination parameters with defaults.
// Default: page=1, pageSize=20, max pageSize=100
//
// Out-of-range values are an error rather than being clamped, so that a client
// asking for pageSize: 1000 finds out its request was not honoured. Clamping
// also left the cap unenforced wherever a resolver read the arguments directly
// instead of calling this, which is how searchPoems ended up able to request an
// unbounded number of rows.
func parsePagination(page, pageSize *int) (Pagination, error) {
	p := defaultPage
	if page != nil {
		if *page < 1 {
			return Pagination{}, fmt.Errorf("page must be at least 1, got %d", *page)
		}
		p = *page
	}

	ps := defaultPageSize
	if pageSize != nil {
		if *pageSize < 1 || *pageSize > maxPageSize {
			return Pagination{}, fmt.Errorf("pageSize must be between 1 and %d, got %d", maxPageSize, *pageSize)
		}
		ps = *pageSize
	}

	return Pagination{
		Page:     p,
		PageSize: ps,
		Offset:   (p - 1) * ps,
	}, nil
}

// parseOptionalID parses an optional string ID to int64 pointer.
// Uses common helper function.
func parseOptionalID(id *string) (*int64, error) {
	return helpers.ParseOptionalInt64(id)
}

// parseLang converts an optional Lang pointer to a Lang value.
// Uses common helper function.
func parseLang(lang *database.Lang) database.Lang {
	return helpers.ParseLangPointer(lang)
}

// buildPoemConnection creates a PoemConnection from poems slice and pagination info.
func buildPoemConnection(poems []database.Poem, pag Pagination, totalCount int) *database.PoemConnection {
	edges := make([]database.PoemEdge, len(poems))
	for i, poem := range poems {
		edges[i] = database.PoemEdge{
			Node:   poem,
			Cursor: strconv.Itoa(pag.Offset + i),
		}
	}

	hasNextPage := pag.Offset+len(poems) < totalCount
	hasPreviousPage := pag.Page > 1

	var startCursor, endCursor *string
	if len(edges) > 0 {
		start := edges[0].Cursor
		end := edges[len(edges)-1].Cursor
		startCursor = &start
		endCursor = &end
	}

	return &database.PoemConnection{
		Edges: edges,
		PageInfo: database.PageInfo{
			HasNextPage:     hasNextPage,
			HasPreviousPage: hasPreviousPage,
			StartCursor:     startCursor,
			EndCursor:       endCursor,
		},
		TotalCount: totalCount,
	}
}

// buildAuthorConnection creates an AuthorConnection from authors slice and pagination info.
func buildAuthorConnection(authors []database.AuthorWithStats, pag Pagination, totalCount int) *database.AuthorConnection {
	edges := make([]database.AuthorEdge, len(authors))
	for i, author := range authors {
		edges[i] = database.AuthorEdge{
			Node:   author,
			Cursor: strconv.Itoa(pag.Offset + i),
		}
	}

	hasNextPage := pag.Offset+len(authors) < totalCount
	hasPreviousPage := pag.Page > 1

	var startCursor, endCursor *string
	if len(edges) > 0 {
		start := edges[0].Cursor
		end := edges[len(edges)-1].Cursor
		startCursor = &start
		endCursor = &end
	}

	return &database.AuthorConnection{
		Edges: edges,
		PageInfo: database.PageInfo{
			HasNextPage:     hasNextPage,
			HasPreviousPage: hasPreviousPage,
			StartCursor:     startCursor,
			EndCursor:       endCursor,
		},
		TotalCount: totalCount,
	}
}
