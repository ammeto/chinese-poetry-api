package handler

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Query parameter names accepted across the REST API.
const (
	queryLang      = "lang"
	queryPage      = "page"
	queryPageSize  = "page_size"
	queryQuery     = "q"
	queryChar      = "char"
	queryAuthorID  = "author_id"
	queryAuthor    = "author"
	queryTypeID    = "type_id"
	queryDynastyID = "dynasty_id"
	queryDynasty   = "dynasty"

	// queryType names the poetry type filter on /poems and /poems/random, and
	// the search mode on /poems/search - two unrelated meanings, one spelling.
	queryType = "type"
)

// checkQueryParams responds 400 and returns false if the request carries any
// query parameter outside allowed.
//
// Gin drops unrecognised query params silently, so before this check a
// misspelled filter (dynastyId, the GraphQL spelling, instead of dynasty_id)
// produced a perfectly normal 200 over the unfiltered corpus - the client had
// no way to tell the filter had been ignored. Every endpoint therefore declares
// the keys it actually reads.
func checkQueryParams(c *gin.Context, allowed ...string) bool {
	var unknown []string
	for key := range c.Request.URL.Query() {
		if !slices.Contains(allowed, key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return true
	}

	// Map iteration order is random; sort so the message is reproducible.
	slices.Sort(unknown)
	sortedAllowed := slices.Clone(allowed)
	slices.Sort(sortedAllowed)

	respondError(c, http.StatusBadRequest, "unknown query parameter(s): "+strings.Join(unknown, ", ")+
		"; supported: "+strings.Join(sortedAllowed, ", "))
	return false
}

// parseIntQuery reads key as an integer within [min, max], returning def when
// the parameter is absent. It responds 400 and returns false for a value that
// is not an integer or falls outside the range, rather than silently coercing
// it - a clamped page_size=1000 or a discarded page=abc used to look like a
// successful request.
func parseIntQuery(c *gin.Context, key string, def, minValue, maxValue int) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return def, true
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		respondError(c, http.StatusBadRequest, key+" must be an integer, got "+strconv.Quote(raw))
		return 0, false
	}
	if value < minValue || value > maxValue {
		respondError(c, http.StatusBadRequest, key+" must be between "+strconv.Itoa(minValue)+
			" and "+strconv.Itoa(maxValue)+", got "+strconv.Itoa(value))
		return 0, false
	}
	return value, true
}

// parseInt64Query reads key as an int64 id, returning nil when absent. It
// responds 400 and returns false for a non-numeric value; previously such a
// value was discarded, silently widening the query to the whole corpus.
func parseInt64Query(c *gin.Context, key string) (*int64, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, key+" must be an integer, got "+strconv.Quote(raw))
		return nil, false
	}
	return &id, true
}
