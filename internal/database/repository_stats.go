package database

// Statistics and counting methods

// CountPoems returns the total number of poems
func (r *Repository) CountPoems() (int, error) {
	var count int64
	err := r.db.Table(r.poemsTable()).Count(&count).Error
	return int(count), err
}

// CountAuthors returns the total number of authors
func (r *Repository) CountAuthors() (int, error) {
	var count int64
	err := r.db.Table(r.authorsTable()).Count(&count).Error
	return int(count), err
}

// CountPoemsByAuthor returns the number of poems attributed to an author.
func (r *Repository) CountPoemsByAuthor(authorID int64) (int, error) {
	return r.countPoemsWhere("author_id = ?", authorID)
}

// CountPoemsByDynasty returns the number of poems from a dynasty.
func (r *Repository) CountPoemsByDynasty(dynastyID int64) (int, error) {
	return r.countPoemsWhere("dynasty_id = ?", dynastyID)
}

// CountPoemsByType returns the number of poems of a poetry type.
func (r *Repository) CountPoemsByType(typeID int64) (int, error) {
	return r.countPoemsWhere("type_id = ?", typeID)
}

// CountAuthorsByDynasty returns the number of distinct authors with at least
// one poem in a dynasty.
func (r *Repository) CountAuthorsByDynasty(dynastyID int64) (int, error) {
	var count int64
	err := r.db.Table(r.poemsTable()).
		Where("dynasty_id = ?", dynastyID).
		Distinct("author_id").
		Count(&count).Error
	return int(count), err
}

// countPoemsWhere counts poems matching a single condition. It exists so these
// counts go through poemsTable() like every other query: the GraphQL resolvers
// used to count via db.Model(&Poem{}), which resolves to Poem.TableName() -
// the legacy unsuffixed "poems", a table that no longer exists after the
// language-variant split, so every one of those fields failed at runtime.
func (r *Repository) countPoemsWhere(query string, args ...any) (int, error) {
	var count int64
	err := r.db.Table(r.poemsTable()).Where(query, args...).Count(&count).Error
	return int(count), err
}

// GetStatistics returns overall statistics
func (r *Repository) GetStatistics() (*Statistics, error) {
	stats := &Statistics{}

	// Total counts
	var err error
	stats.TotalPoems, err = r.CountPoems()
	if err != nil {
		return nil, err
	}

	stats.TotalAuthors, err = r.CountAuthors()
	if err != nil {
		return nil, err
	}

	var count int64
	err = r.db.Table(r.dynastiesTable()).Where("name != ?", "其他").Count(&count).Error
	if err != nil {
		return nil, err
	}
	stats.TotalDynasties = int(count)

	// Poems by dynasty - use raw SQL with dynamic table names
	dynastyTable := r.dynastiesTable()
	poemTable := r.poemsTable()

	var dynastyStats []struct {
		Dynasty
		PoemCount int `gorm:"column:poem_count"`
	}

	err = r.db.Table(dynastyTable).
		Select(dynastyTable + ".*, COUNT(" + poemTable + ".id) as poem_count").
		Joins("LEFT JOIN " + poemTable + " ON " + dynastyTable + ".id = " + poemTable + ".dynasty_id").
		Group(dynastyTable + ".id").
		Order("poem_count DESC").
		Scan(&dynastyStats).Error
	if err != nil {
		return nil, err
	}

	for _, ds := range dynastyStats {
		stats.PoemsByDynasty = append(stats.PoemsByDynasty, DynastyWithStats{
			Dynasty:   ds.Dynasty,
			PoemCount: ds.PoemCount,
		})
	}

	// Poems by type
	typeTable := r.poetryTypesTable()

	var typeStats []struct {
		PoetryType
		PoemCount int `gorm:"column:poem_count"`
	}

	err = r.db.Table(typeTable).
		Select(typeTable + ".*, COUNT(" + poemTable + ".id) as poem_count").
		Joins("LEFT JOIN " + poemTable + " ON " + typeTable + ".id = " + poemTable + ".type_id").
		Group(typeTable + ".id").
		Order("poem_count DESC").
		Scan(&typeStats).Error
	if err != nil {
		return nil, err
	}

	for _, ts := range typeStats {
		stats.PoemsByType = append(stats.PoemsByType, PoetryTypeWithStats{
			PoetryType: ts.PoetryType,
			PoemCount:  ts.PoemCount,
		})
	}

	return stats, nil
}

// ListAuthorsWithFilter returns a paginated list of authors with optional dynasty filter
func (r *Repository) ListAuthorsWithFilter(limit, offset int, dynastyID *int64) ([]AuthorWithStats, int, error) {
	authorTable := r.authorsTable()
	poemTable := r.poemsTable()

	query := r.db.Table(authorTable)

	// Apply dynasty filter
	if dynastyID != nil {
		query = query.Where(authorTable+".dynasty_id = ?", *dynastyID)
	}

	// Get total count
	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	// Get authors with poem counts
	var results []struct {
		Author
		PoemCount int `gorm:"column:poem_count"`
	}

	err := query.
		Select(authorTable + ".*, COUNT(" + poemTable + ".id) as poem_count").
		Joins("LEFT JOIN " + poemTable + " ON " + authorTable + ".id = " + poemTable + ".author_id").
		Group(authorTable + ".id").
		Order("poem_count DESC").
		Limit(limit).Offset(offset).
		Scan(&results).Error
	if err != nil {
		return nil, 0, err
	}

	// Convert to AuthorWithStats
	authors := make([]AuthorWithStats, len(results))
	for i, r := range results {
		authors[i] = AuthorWithStats{
			Author:    r.Author,
			PoemCount: r.PoemCount,
		}
	}

	return authors, int(totalCount), nil
}
