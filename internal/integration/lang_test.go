package integration

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/palemoky/chinese-poetry-api/internal/database"
	"github.com/palemoky/chinese-poetry-api/internal/graph"
	"github.com/palemoky/chinese-poetry-api/internal/graph/generated"
)

// setupLangTestEnv builds a GraphQL client over a file-backed database.
//
// A file is used rather than ":memory:" because each SQLite connection to
// ":memory:" gets its own private database, so once gqlgen resolves fields
// concurrently the extra pool connections see an unmigrated, empty schema and
// queries fail with spurious "no such table" errors.
func setupLangTestEnv(t *testing.T) (*client.Client, *database.Repository) {
	gormDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	require.NoError(t, err)

	db := &database.DB{DB: gormDB}
	require.NoError(t, db.Migrate())

	repo := database.NewRepository(db)
	resolver := graph.NewResolver(db, repo)
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))

	return client.New(srv), repo
}

// seedVariant writes one poem with a variant-specific title so a query that
// reads the wrong table is visible in the result rather than merely plausible.
func seedVariant(t *testing.T, repo *database.Repository, lang database.Lang, title, author string) {
	langRepo := repo.WithLang(lang)

	dynastyID, err := langRepo.GetOrCreateDynasty("唐")
	require.NoError(t, err)
	authorID, err := langRepo.GetOrCreateAuthor(author, dynastyID)
	require.NoError(t, err)

	require.NoError(t, langRepo.InsertPoem(&database.Poem{
		ID:        1,
		Title:     title,
		Content:   datatypes.JSON([]byte(`["床前明月光"]`)),
		AuthorID:  &authorID,
		DynastyID: &dynastyID,
	}))
}

// TestGraphQLLangSelectsVariant covers the lang argument, which used to have no
// effect anywhere in the GraphQL API. Two defects combined: gqlgen's autobind
// turned the enum literal straight into database.Lang("ZH_HANT"), a value equal
// to neither variant constant so every table helper fell through to simplified;
// and poems/poem/authors/author never called WithLang at all.
func TestGraphQLLangSelectsVariant(t *testing.T) {
	c, repo := setupLangTestEnv(t)
	seedVariant(t, repo, database.LangHans, "简体标题", "李白")
	seedVariant(t, repo, database.LangHant, "繁體標題", "李白傳統")

	t.Run("poems", func(t *testing.T) {
		for lang, want := range map[string]string{"ZH_HANS": "简体标题", "ZH_HANT": "繁體標題"} {
			var resp struct {
				Poems struct {
					Edges []struct {
						Node struct{ Title string }
					}
				}
			}
			require.NoError(t, c.Post(`query { poems(lang: `+lang+`) { edges { node { title } } } }`, &resp))
			require.Len(t, resp.Poems.Edges, 1)
			assert.Equal(t, want, resp.Poems.Edges[0].Node.Title, "lang: %s read the wrong table", lang)
		}
	})

	t.Run("poem by id", func(t *testing.T) {
		for lang, want := range map[string]string{"ZH_HANS": "简体标题", "ZH_HANT": "繁體標題"} {
			var resp struct {
				Poem struct{ Title string }
			}
			require.NoError(t, c.Post(`query { poem(id: "1", lang: `+lang+`) { title } }`, &resp))
			assert.Equal(t, want, resp.Poem.Title)
		}
	})

	t.Run("authors", func(t *testing.T) {
		for lang, want := range map[string]string{"ZH_HANS": "李白", "ZH_HANT": "李白傳統"} {
			var resp struct {
				Authors struct {
					Edges []struct {
						Node struct{ Name string }
					}
				}
			}
			require.NoError(t, c.Post(`query { authors(lang: `+lang+`) { edges { node { name } } } }`, &resp))
			require.Len(t, resp.Authors.Edges, 1)
			assert.Equal(t, want, resp.Authors.Edges[0].Node.Name)
		}
	})

	t.Run("searchPoems", func(t *testing.T) {
		var resp struct {
			SearchPoems struct{ TotalCount int }
		}
		require.NoError(t, c.Post(`query { searchPoems(query: "繁體", lang: ZH_HANT) { totalCount } }`, &resp))
		assert.Equal(t, 1, resp.SearchPoems.TotalCount)

		require.NoError(t, c.Post(`query { searchPoems(query: "繁體", lang: ZH_HANS) { totalCount } }`, &resp))
		assert.Equal(t, 0, resp.SearchPoems.TotalCount, "simplified table has no traditional title")
	})

	t.Run("an unknown enum literal is rejected", func(t *testing.T) {
		var resp struct{}
		err := c.Post(`query { poems(lang: ZH_HANZ) { totalCount } }`, &resp)
		require.Error(t, err)
	})
}

// TestSearchPoemsCursors covers searchPoems' connection, which it used to build
// by hand: cursors were numbered from 0 within each page, so page 2's first edge
// carried the same cursor as page 1's, and startCursor/endCursor were left unset
// even though every other connection populates them.
func TestSearchPoemsCursors(t *testing.T) {
	c, repo := setupLangTestEnv(t)

	dynastyID, err := repo.GetOrCreateDynasty("唐")
	require.NoError(t, err)
	authorID, err := repo.GetOrCreateAuthor("李白", dynastyID)
	require.NoError(t, err)
	for i := range 4 {
		require.NoError(t, repo.InsertPoem(&database.Poem{
			ID:        int64(i + 1),
			Title:     "春日" + string(rune('A'+i)),
			Content:   datatypes.JSON([]byte(`["春风"]`)),
			AuthorID:  &authorID,
			DynastyID: &dynastyID,
		}))
	}

	page := func(n int) (cursors []string, start, end *string) {
		var resp struct {
			SearchPoems struct {
				Edges []struct {
					Cursor string
					Node   struct{ Title string }
				}
				PageInfo struct {
					StartCursor *string
					EndCursor   *string
				}
			}
		}
		q := `query { searchPoems(query: "春日", page: ` + strconv.Itoa(n) + `, pageSize: 2) {
			edges { cursor node { title } }
			pageInfo { startCursor endCursor }
		} }`
		require.NoError(t, c.Post(q, &resp))

		for _, e := range resp.SearchPoems.Edges {
			cursors = append(cursors, e.Cursor)
		}
		return cursors, resp.SearchPoems.PageInfo.StartCursor, resp.SearchPoems.PageInfo.EndCursor
	}

	first, start1, end1 := page(1)
	second, start2, _ := page(2)

	assert.Equal(t, []string{"0", "1"}, first)
	assert.Equal(t, []string{"2", "3"}, second, "page 2 cursors must continue from page 1, not restart at 0")

	require.NotNil(t, start1)
	require.NotNil(t, end1)
	require.NotNil(t, start2)
	assert.Equal(t, "0", *start1)
	assert.Equal(t, "1", *end1)
	assert.Equal(t, "2", *start2)
}

// TestAuthorListingTieBreak covers paging over authors whose poem_count ties.
// Ordering by poem_count alone is not a total order, so which of the tied rows
// a given LIMIT/OFFSET window returns is unspecified.
func TestAuthorListingTieBreak(t *testing.T) {
	c, repo := setupLangTestEnv(t)

	dynastyID, err := repo.GetOrCreateDynasty("唐")
	require.NoError(t, err)

	// Every author gets exactly one poem, so poem_count ties across all of them.
	const authorCount = 12
	for i := range authorCount {
		name := "作者" + strconv.Itoa(i)
		authorID, err := repo.GetOrCreateAuthor(name, dynastyID)
		require.NoError(t, err)
		require.NoError(t, repo.InsertPoem(&database.Poem{
			ID:        int64(i + 1),
			Title:     "诗" + strconv.Itoa(i),
			Content:   datatypes.JSON([]byte(`["内容"]`)),
			AuthorID:  &authorID,
			DynastyID: &dynastyID,
		}))
	}

	seen := map[string]int{}
	for page := 1; page <= authorCount/3; page++ {
		var resp struct {
			Authors struct {
				Edges []struct {
					Node struct{ Name string }
				}
			}
		}
		q := `query { authors(page: ` + strconv.Itoa(page) + `, pageSize: 3) { edges { node { name } } } }`
		require.NoError(t, c.Post(q, &resp))
		for _, e := range resp.Authors.Edges {
			seen[e.Node.Name]++
		}
	}

	require.Len(t, seen, authorCount, "paging must visit every author exactly once")
	for name, n := range seen {
		assert.Equal(t, 1, n, "%s appeared on more than one page", name)
	}
}

// TestGraphQLCountFields covers the poemCount/authorCount fields, which counted
// via db.Model(&Poem{}). That resolves to Poem.TableName() - the legacy
// unsuffixed "poems", dropped when the language-variant tables were introduced -
// so every one of these fields failed with "no such table: poems".
func TestGraphQLCountFields(t *testing.T) {
	c, repo := setupLangTestEnv(t)
	seedVariant(t, repo, database.LangHans, "简体标题", "李白")

	t.Run("dynasty counts", func(t *testing.T) {
		var resp struct {
			Dynasties []struct {
				Name        string
				PoemCount   int
				AuthorCount int
			}
		}
		require.NoError(t, c.Post(`query { dynasties { name poemCount authorCount } }`, &resp))

		var found bool
		for _, d := range resp.Dynasties {
			if d.Name == "唐" {
				found = true
				assert.Equal(t, 1, d.PoemCount)
				assert.Equal(t, 1, d.AuthorCount)
			}
		}
		assert.True(t, found, "seeded dynasty missing from result")
	})

	t.Run("author poem count", func(t *testing.T) {
		var resp struct {
			Authors struct {
				Edges []struct {
					Node struct {
						Name      string
						PoemCount int
					}
				}
			}
		}
		require.NoError(t, c.Post(`query { authors { edges { node { name poemCount } } } }`, &resp))
		require.Len(t, resp.Authors.Edges, 1)
		assert.Equal(t, 1, resp.Authors.Edges[0].Node.PoemCount)
	})

	t.Run("poetry type poem count", func(t *testing.T) {
		var resp struct {
			PoemTypes []struct {
				Name      string
				PoemCount int
			}
		}
		require.NoError(t, c.Post(`query { poemTypes { name poemCount } }`, &resp))
		assert.NotEmpty(t, resp.PoemTypes, "poetry types are seeded by the schema")
	})
}
