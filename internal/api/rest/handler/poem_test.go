package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/palemoky/chinese-poetry-api/internal/database"
)

// setupPoemTestRouter creates a test router with database
func setupPoemTestRouter(t *testing.T) (*gin.Engine, *database.Repository) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	db := &database.DB{DB: gormDB}

	// Use Migrate() to create language-specific tables
	err = db.Migrate()
	require.NoError(t, err)

	repo := database.NewRepository(db)

	router := gin.New()
	return router, repo
}

// createTestPoem creates a test poem in the database
func createTestPoem(t *testing.T, repo *database.Repository, id int64, title, content string) *database.Poem {
	// Create dynasty and author first
	dynastyID, err := repo.GetOrCreateDynasty("唐")
	require.NoError(t, err)

	authorID, err := repo.GetOrCreateAuthor("李白", dynastyID)
	require.NoError(t, err)

	// Create poem
	poem := &database.Poem{
		ID:        id,
		Title:     title,
		Content:   datatypes.JSON([]byte(`["床前明月光","疑是地上霜","举头望明月","低头思故乡"]`)),
		AuthorID:  &authorID,
		DynastyID: &dynastyID,
	}
	err = repo.InsertPoem(poem)
	require.NoError(t, err)

	return poem
}

func TestListPoems(t *testing.T) {
	router, repo := setupPoemTestRouter(t)
	handler := NewPoemHandler(repo)

	// Create test poems
	createTestPoem(t, repo, 1, "静夜思", "test content")
	createTestPoem(t, repo, 2, "春晓", "test content 2")

	router.GET("/poems", handler.ListPoems)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]any)
	}{
		{
			name:           "list poems default pagination",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]any) {
				data := resp["data"].([]any)
				assert.Len(t, data, 2)

				pagination := resp["pagination"].(map[string]any)
				assert.Equal(t, float64(1), pagination["page"])
				assert.Equal(t, float64(20), pagination["page_size"])
				assert.Equal(t, float64(2), pagination["total"])

				// Check nested structure of first poem
				poem := data[0].(map[string]any)
				assert.NotEmpty(t, poem["title"])
				assert.NotEmpty(t, poem["content"])

				assert.NotNil(t, poem["author"])
				author := poem["author"].(map[string]any)
				assert.Equal(t, "李白", author["name"])

				assert.NotNil(t, poem["dynasty"])
				dynasty := poem["dynasty"].(map[string]any)
				assert.Equal(t, "唐", dynasty["name"])
			},
		},
		{
			name:           "list poems with pagination",
			query:          "?page=1&page_size=1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]any) {
				data := resp["data"].([]any)
				assert.Len(t, data, 1) // Should only return 1

				pagination := resp["pagination"].(map[string]any)
				assert.Equal(t, float64(1), pagination["page"])
				assert.Equal(t, float64(1), pagination["page_size"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/poems"+tt.query, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]any
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}

// TestListPoemsFilters covers the filters on /poems, which previously accepted
// every spelling and silently ignored all of them: dynasty_id=6, dynasty=唐 and
// dynastyId=6 all returned 200 over the unfiltered corpus, so a client had no
// way to notice it had got the parameter wrong.
func TestListPoemsFilters(t *testing.T) {
	router, repo := setupPoemTestRouter(t)
	handler := NewPoemHandler(repo)

	tangID, err := repo.GetOrCreateDynasty("唐")
	require.NoError(t, err)
	songID, err := repo.GetOrCreateDynasty("宋")
	require.NoError(t, err)
	libaiID, err := repo.GetOrCreateAuthor("李白", tangID)
	require.NoError(t, err)
	sushiID, err := repo.GetOrCreateAuthor("苏轼", songID)
	require.NoError(t, err)

	// Poetry types are pre-seeded by Migrate; 11 and 12 are 五言绝句/七言绝句.
	jueju5, jueju7 := int64(11), int64(12)
	poems := []*database.Poem{
		{ID: 1, Title: "静夜思", AuthorID: &libaiID, DynastyID: &tangID, TypeID: &jueju5},
		{ID: 2, Title: "早发白帝城", AuthorID: &libaiID, DynastyID: &tangID, TypeID: &jueju7},
		{ID: 3, Title: "题西林壁", AuthorID: &sushiID, DynastyID: &songID, TypeID: &jueju7},
	}
	for _, p := range poems {
		p.Content = datatypes.JSON([]byte(`["内容"]`))
		require.NoError(t, repo.InsertPoem(p))
	}

	router.GET("/poems", handler.ListPoems)

	// titles runs a request and returns the titles it produced, so each case
	// asserts on which poems came back rather than only on the status code.
	titles := func(t *testing.T, query string, wantStatus int) []string {
		req := httptest.NewRequest(http.MethodGet, "/poems"+query, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, wantStatus, w.Code, "body: %s", w.Body.String())

		if wantStatus != http.StatusOK {
			return nil
		}

		var resp struct {
			Data []struct {
				Title string `json:"title"`
			} `json:"data"`
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		got := make([]string, len(resp.Data))
		for i, d := range resp.Data {
			got[i] = d.Title
		}
		// The reported total must describe the filtered set, not the corpus.
		assert.Equal(t, len(got), resp.Pagination.Total)
		return got
	}

	t.Run("filters actually filter", func(t *testing.T) {
		assert.Equal(t, []string{"静夜思", "早发白帝城", "题西林壁"}, titles(t, "", http.StatusOK))
		assert.Equal(t, []string{"静夜思", "早发白帝城"}, titles(t, "?dynasty_id="+strconv.FormatInt(tangID, 10), http.StatusOK))
		assert.Equal(t, []string{"静夜思", "早发白帝城"}, titles(t, "?dynasty=唐", http.StatusOK))
		assert.Equal(t, []string{"题西林壁"}, titles(t, "?author=苏轼", http.StatusOK))
		assert.Equal(t, []string{"静夜思"}, titles(t, "?author=李白&type_id=11", http.StatusOK))
	})

	t.Run("repeated type_id is combined with OR", func(t *testing.T) {
		assert.Equal(t, []string{"静夜思", "早发白帝城", "题西林壁"}, titles(t, "?type_id=11&type_id=12", http.StatusOK))
		assert.Equal(t, []string{"早发白帝城", "题西林壁"}, titles(t, "?type_id=12", http.StatusOK))
	})

	t.Run("misspelled and malformed filters are rejected", func(t *testing.T) {
		// The GraphQL spelling leaking into REST is the motivating case.
		titles(t, "?dynastyId=6", http.StatusBadRequest)
		titles(t, "?dynasty_ids=6", http.StatusBadRequest)
		titles(t, "?dynasty_id=abc", http.StatusBadRequest)
		titles(t, "?type_id=abc", http.StatusBadRequest)
		titles(t, "?author_id=1.5", http.StatusBadRequest)
		titles(t, "?lang=en", http.StatusBadRequest)
	})

	t.Run("unknown filter values are 404, not a silent full listing", func(t *testing.T) {
		titles(t, "?dynasty=不存在的朝代", http.StatusNotFound)
		titles(t, "?author=不存在的作者", http.StatusNotFound)
	})

	t.Run("a known filter value with no poems is an empty page, not a 404", func(t *testing.T) {
		// 元 is seeded by the schema but has no poems in this fixture.
		assert.Empty(t, titles(t, "?dynasty=元", http.StatusOK))
	})
}

func TestSearchPoems(t *testing.T) {
	router, repo := setupPoemTestRouter(t)
	handler := NewPoemHandler(repo)

	// Create test poems
	createTestPoem(t, repo, 1, "静夜思", "test content")

	router.GET("/poems/search", handler.SearchPoems)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]any)
	}{
		{
			name:           "search with query",
			query:          "?q=静夜思",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]any) {
				assert.Contains(t, resp, "data")
				assert.Contains(t, resp, "pagination")

				pagination := resp["pagination"].(map[string]any)
				assert.Contains(t, pagination, "total")
				assert.Contains(t, pagination, "page")
				assert.Contains(t, pagination, "page_size")
			},
		},
		{
			name:           "search with type parameter",
			query:          "?q=李白&type=author",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]any) {
				assert.Contains(t, resp, "data")
			},
		},
		{
			name:           "search with pagination",
			query:          "?q=test&page=1&page_size=10",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]any) {
				pagination := resp["pagination"].(map[string]any)
				assert.Equal(t, float64(1), pagination["page"])
				assert.Equal(t, float64(10), pagination["page_size"])
			},
		},
		{
			name:           "search without query parameter",
			query:          "",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]any) {
				assert.Equal(t, "query parameter 'q' is required", resp["error"])
			},
		},
		{
			name:           "page_size exceeds limit",
			query:          "?q=test&page_size=200",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]any) {
				assert.Contains(t, resp["error"], "page_size")
			},
		},
		{
			// Unknown search types fell through to "all", so a typo searched
			// everything and looked like a working narrow search.
			name:           "unknown search type is rejected",
			query:          "?q=李白&type=titel",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]any) {
				assert.Contains(t, resp["error"], "titel")
			},
		},
		{
			name:           "unknown query parameter is rejected",
			query:          "?q=李白&searchType=author",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, resp map[string]any) {
				assert.Contains(t, resp["error"], "searchType")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/poems/search"+tt.query, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]any
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}

func TestRandomPoem(t *testing.T) {
	router, repo := setupPoemTestRouter(t)
	handler := NewPoemHandler(repo)

	router.GET("/random", handler.RandomPoem)

	tests := []struct {
		name           string
		query          string
		setupData      bool
		expectedStatus int
		checkResponse  func(*testing.T, map[string]any)
	}{
		{
			name:           "get random poem when database is empty",
			query:          "",
			setupData:      false,
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, resp map[string]any) {
				assert.Equal(t, "no poems found matching the criteria", resp["error"])
			},
		},
		{
			name:           "get random poem with data",
			query:          "",
			setupData:      true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]any) {
				assert.NotEmpty(t, resp["title"])
				assert.NotEmpty(t, resp["content"])

				assert.NotNil(t, resp["author"])
				author := resp["author"].(map[string]any)
				assert.Equal(t, "李白", author["name"])

				assert.NotNil(t, resp["dynasty"])
				dynasty := resp["dynasty"].(map[string]any)
				assert.Equal(t, "唐", dynasty["name"])
			},
		},
		{
			name:           "get random poem with author filter",
			query:          "?author=李白",
			setupData:      true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]any) {
				assert.NotEmpty(t, resp["title"])
				author := resp["author"].(map[string]any)
				assert.Equal(t, "李白", author["name"])
			},
		},
		{
			name:           "get random poem with non-existent author filter",
			query:          "?author=不存在的作者",
			setupData:      true,
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, resp map[string]any) {
				assert.Equal(t, "author not found", resp["error"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh router for each test to avoid data pollution
			router, repo := setupPoemTestRouter(t)
			handler := NewPoemHandler(repo)
			router.GET("/random", handler.RandomPoem)

			if tt.setupData {
				createTestPoem(t, repo, 1, "静夜思", "test content")
			}

			req := httptest.NewRequest(http.MethodGet, "/random"+tt.query, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				var response map[string]any
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}
