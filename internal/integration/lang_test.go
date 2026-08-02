package integration

import (
	"path/filepath"
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
