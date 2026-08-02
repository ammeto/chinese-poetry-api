package database

import (
	"testing"

	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupBenchDB 创建基准测试用的内存数据库。
func setupBenchDB(b *testing.B) (*DB, *Repository) {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatal(err)
	}

	err = gormDB.AutoMigrate(&Dynasty{}, &Author{}, &PoetryType{}, &Poem{})
	if err != nil {
		b.Fatal(err)
	}

	db := &DB{DB: gormDB}
	repo := NewRepository(db)
	return db, repo
}

// BenchmarkGetPoemByID 测量单首诗词查询的性能。
func BenchmarkGetPoemByID(b *testing.B) {
	_, repo := setupBenchDB(b)

	// 写入测试数据
	dynastyID, _ := repo.GetOrCreateDynasty("唐")
	authorID, _ := repo.GetOrCreateAuthor("李白", dynastyID)

	poem := &Poem{
		ID:        1,
		Title:     "静夜思",
		Content:   datatypes.JSON([]byte(`["床前明月光","疑是地上霜","举头望明月","低头思故乡"]`)),
		AuthorID:  &authorID,
		DynastyID: &dynastyID,
	}
	_ = repo.InsertPoem(poem)

	for b.Loop() {
		_, _ = repo.GetPoemByID("1")
	}
}

// BenchmarkListPoems 测量分页列表查询的性能。
func BenchmarkListPoems(b *testing.B) {
	_, repo := setupBenchDB(b)

	// 写入测试数据
	dynastyID, _ := repo.GetOrCreateDynasty("唐")
	authorID, _ := repo.GetOrCreateAuthor("李白", dynastyID)

	// 准备待写入的诗词
	poems := make([]*Poem, 100)
	for i := range 100 {
		poems[i] = &Poem{
			ID:        int64(i + 1), // 简单的顺序 ID
			Title:     "测试诗词" + string(rune('A'+i%26)),
			Content:   datatypes.JSON([]byte(`["测试内容"]`)),
			AuthorID:  &authorID,
			DynastyID: &dynastyID,
		}
		_ = repo.InsertPoem(poems[i])
	}

	testCases := []struct {
		name     string
		page     int
		pageSize int
	}{
		{"small_page", 1, 10},
		{"medium_page", 1, 20},
		{"large_page", 1, 50},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for b.Loop() {
				_, _, _ = repo.ListPoemsWithFilter(tc.pageSize, (tc.page-1)*tc.pageSize, nil, nil, nil)
			}
		})
	}
}

// BenchmarkGetOrCreateAuthor 测量作者查询或创建的性能。
func BenchmarkGetOrCreateAuthor(b *testing.B) {
	_, repo := setupBenchDB(b)

	dynastyID, _ := repo.GetOrCreateDynasty("唐")

	testCases := []struct {
		name   string
		author string
	}{
		{"new", "李白"},
		{"existing", "李白"},
	}

	// 预先写入，供「已存在」的分支使用
	_, _ = repo.GetOrCreateAuthor("李白", dynastyID)

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for b.Loop() {
				_, _ = repo.GetOrCreateAuthor(tc.author, dynastyID)
			}
		})
	}
}

// BenchmarkInsertPoem 测量诗词写入的性能。
func BenchmarkInsertPoem(b *testing.B) {
	_, repo := setupBenchDB(b)

	dynastyID, _ := repo.GetOrCreateDynasty("唐")
	authorID, _ := repo.GetOrCreateAuthor("李白", dynastyID)

	for i := range b.N {
		b.StopTimer()
		poem := &Poem{
			ID:        int64(i + 1),
			Title:     "静夜思",
			Content:   datatypes.JSON([]byte(`["床前明月光","疑是地上霜"]`)),
			AuthorID:  &authorID,
			DynastyID: &dynastyID,
		}
		b.StartTimer()
		_ = repo.InsertPoem(poem)
	}
}

// BenchmarkGetAuthorByID 测量按 ID 查询作者的性能。
func BenchmarkGetAuthorByID(b *testing.B) {
	_, repo := setupBenchDB(b)

	dynastyID, _ := repo.GetOrCreateDynasty("唐")
	authorID, _ := repo.GetOrCreateAuthor("李白", dynastyID)

	b.ResetTimer()
	for b.Loop() {
		_, _ = repo.GetAuthorByID(authorID)
	}
}

// BenchmarkGetRandomPoemMultipleTypes 测量多体裁过滤下随机取词的性能。
func BenchmarkGetRandomPoemMultipleTypes(b *testing.B) {
	_, repo := setupBenchDB(b)

	// 写入测试数据
	dynastyID, _ := repo.GetOrCreateDynasty("唐")
	authorID, _ := repo.GetOrCreateAuthor("李白", dynastyID)

	// 写入多个体裁
	typeNames := []string{"五言绝句", "七言绝句", "五言律诗", "七言律诗"}
	typeIDs := make([]int64, len(typeNames))
	for i, typeName := range typeNames {
		ptype := PoetryType{Name: typeName}
		_ = repo.db.Table(repo.poetryTypesTable()).Create(&ptype).Error
		typeIDs[i] = ptype.ID
	}

	// 为每个体裁写入诗词
	for i, typeID := range typeIDs {
		for j := 0; j < 100; j++ {
			poem := &Poem{
				ID:        int64(i*100 + j + 1),
				Title:     typeNames[i] + "测试",
				Content:   datatypes.JSON([]byte(`["测试内容"]`)),
				AuthorID:  &authorID,
				DynastyID: &dynastyID,
				TypeID:    &typeID,
			}
			_ = repo.InsertPoem(poem)
		}
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = repo.GetRandomPoem(&dynastyID, nil, typeIDs)
	}
}
