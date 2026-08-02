package handler

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/palemoky/chinese-poetry-api/internal/database"
)

// PoemHandler handles poem-related requests
type PoemHandler struct {
	repo *database.Repository
}

// NewPoemHandler creates a new poem handler
func NewPoemHandler(repo *database.Repository) *PoemHandler {
	return &PoemHandler{
		repo: repo,
	}
}

// ListPoems retrieves a paginated list of poems
// Supports ?lang=zh-Hans (default) or ?lang=zh-Hant
// Supports the same filters as RandomPoem: ?author=李白&type=五言绝句&dynasty=唐
// Or by ID: ?author_id=123&type_id=456&type_id=789&dynasty_id=6
func (h *PoemHandler) ListPoems(c *gin.Context) {
	if !checkQueryParams(c, append([]string{queryLang, queryPage, queryPageSize}, filterQueryKeys...)...) {
		return
	}

	lang, ok := parseLang(c)
	if !ok {
		return
	}
	repo := h.repo.WithLang(lang)

	pagination, ok := ParsePagination(c)
	if !ok {
		return
	}

	filters, ok := parsePoemFilters(c, repo)
	if !ok {
		return
	}

	// Shares ListPoemsWithFilter with the GraphQL poems resolver so both APIs
	// return the same poems in the same order for the same filters.
	poems, total, err := repo.ListPoemsWithFilter(
		pagination.PageSize, pagination.Offset(),
		filters.dynastyID, filters.authorID, filters.typeIDs,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to retrieve poems")
		return
	}

	data := make([]map[string]any, len(poems))
	for i, poem := range poems {
		data[i] = formatPoem(&poem)
	}

	c.JSON(http.StatusOK, NewPaginationResponse(data, pagination, int64(total)))
}

// searchTypes lists the accepted values of the search endpoint's type param.
var searchTypes = []string{"all", "title", "content", "author"}

// SearchPoems searches for poems by query string
func (h *PoemHandler) SearchPoems(c *gin.Context) {
	if !checkQueryParams(c, queryLang, queryPage, queryPageSize, queryQuery, queryType) {
		return
	}

	lang, ok := parseLang(c)
	if !ok {
		return
	}
	repo := h.repo.WithLang(lang)

	query := c.Query(queryQuery)
	if query == "" {
		respondError(c, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	// An unknown search type used to fall through to "all", so ?type=titel
	// searched everything and looked like it had worked.
	searchType := c.DefaultQuery(queryType, "all")
	if !slices.Contains(searchTypes, searchType) {
		respondError(c, http.StatusBadRequest, "unsupported type "+strconv.Quote(searchType)+
			"; supported: "+strings.Join(searchTypes, ", "))
		return
	}

	pagination, ok := ParsePagination(c)
	if !ok {
		return
	}

	// Use repository's search method instead of search engine
	poems, total, err := repo.SearchPoems(query, searchType, pagination.Page, pagination.PageSize)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "search failed")
		return
	}

	data := make([]map[string]any, len(poems))
	for i, poem := range poems {
		data[i] = formatPoem(&poem)
	}

	c.JSON(http.StatusOK, NewPaginationResponse(data, pagination, total))
}

// filterQueryKeys lists the author/type/dynasty filter params shared by
// /poems and /poems/random. Also used to reject char being combined with them
// (see RandomPoem doc comment).
var filterQueryKeys = []string{queryAuthorID, queryAuthor, queryTypeID, queryType, queryDynastyID, queryDynasty}

// poemFilters holds the parsed author/type/dynasty filters. A nil id or an
// empty typeIDs means "no filter on this field".
type poemFilters struct {
	dynastyID *int64
	authorID  *int64
	typeIDs   []int64
}

// parsePoemFilters reads the filters in filterQueryKeys, resolving the name
// variants (?author=李白) to ids via repo. It responds and returns false on a
// malformed id (400) or an unknown name (404).
//
// Each filter accepts either an id or a name; when both are given the id wins.
func parsePoemFilters(c *gin.Context, repo *database.Repository) (poemFilters, bool) {
	var filters poemFilters

	// Author filter (by ID or name)
	authorID, ok := parseInt64Query(c, queryAuthorID)
	if !ok {
		return poemFilters{}, false
	}
	switch {
	case authorID != nil:
		filters.authorID = authorID
	case c.Query(queryAuthor) != "":
		author, err := repo.GetAuthorByName(c.Query(queryAuthor))
		if err != nil {
			respondError(c, http.StatusNotFound, "author not found")
			return poemFilters{}, false
		}
		filters.authorID = &author.ID
	}

	// Type filter (by ID or name) - supports multiple values, combined with OR
	typeIDStrs := c.QueryArray(queryTypeID)
	typeNames := c.QueryArray(queryType)
	switch {
	case len(typeIDStrs) > 0:
		for _, idStr := range typeIDStrs {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				respondError(c, http.StatusBadRequest, queryTypeID+" must be an integer, got "+strconv.Quote(idStr))
				return poemFilters{}, false
			}
			filters.typeIDs = append(filters.typeIDs, id)
		}
	case len(typeNames) > 0:
		// Batch lookup types by name in a single query
		ids, err := repo.GetPoetryTypeIDs(typeNames)
		if err != nil {
			respondError(c, http.StatusNotFound, "poetry type not found")
			return poemFilters{}, false
		}
		filters.typeIDs = ids
	}

	// Dynasty filter (by ID or name)
	dynastyID, ok := parseInt64Query(c, queryDynastyID)
	if !ok {
		return poemFilters{}, false
	}
	switch {
	case dynastyID != nil:
		filters.dynastyID = dynastyID
	case c.Query(queryDynasty) != "":
		dynasty, err := repo.GetDynastyByName(c.Query(queryDynasty))
		if err != nil {
			respondError(c, http.StatusNotFound, "dynasty not found")
			return poemFilters{}, false
		}
		filters.dynastyID = &dynasty.ID
	}

	return filters, true
}

// RandomPoem returns a random poem with optional filters
// Supports ?lang=zh-Hans (default) or ?lang=zh-Hant
// Supports filters: ?author=李白&type=五言绝句&type=七言绝句&dynasty=唐
// Or by ID: ?author_id=123&type_id=456&type_id=789&dynasty_id=789
//
// Supports 飞花令-style single-character search: ?char=春
// char is only combinable with lang - not with author/type/dynasty filters,
// since it selects poems via the FTS content index rather than the id-based
// filters used elsewhere in this handler.
func (h *PoemHandler) RandomPoem(c *gin.Context) {
	if !checkQueryParams(c, append([]string{queryLang, queryChar}, filterQueryKeys...)...) {
		return
	}

	lang, ok := parseLang(c)
	if !ok {
		return
	}
	repo := h.repo.WithLang(lang)

	if char := c.Query(queryChar); char != "" {
		for _, key := range filterQueryKeys {
			if c.Query(key) != "" {
				respondError(c, http.StatusBadRequest, "char cannot be combined with author/type/dynasty filters")
				return
			}
		}
		if utf8.RuneCountInString(char) != 1 {
			respondError(c, http.StatusBadRequest, "char must be a single character")
			return
		}

		poem, err := repo.GetRandomPoemByChar(char)
		if err != nil {
			respondError(c, http.StatusNotFound, "no poems found containing the given character")
			return
		}

		c.JSON(http.StatusOK, formatPoem(poem))
		return
	}

	filters, ok := parsePoemFilters(c, repo)
	if !ok {
		return
	}

	// Get a random poem with filters
	poem, err := repo.GetRandomPoem(filters.dynastyID, filters.authorID, filters.typeIDs)
	if err != nil {
		respondError(c, http.StatusNotFound, "no poems found matching the criteria")
		return
	}

	c.JSON(http.StatusOK, formatPoem(poem))
}
