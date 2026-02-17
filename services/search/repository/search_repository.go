package repository

import (
	"fmt"
	"strings"

	"tachyon-messenger/services/search/models"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
)

// SearchRepository defines the interface for search data access
type SearchRepository interface {
	// Indexing
	UpsertDocument(doc *models.SearchDocument) error
	UpsertDocuments(docs []*models.SearchDocument) error
	DeleteDocument(entityType models.EntityType, entityID uint) error

	// Searching
	Search(query string, userID uint, userRole string, entityType string, limit, offset int) ([]models.SearchResult, int64, error)
	CountByCategories(query string, userID uint, userRole string, types []models.EntityType) (map[models.EntityType]int64, error)
}

type searchRepository struct {
	db *database.DB
}

// NewSearchRepository creates a new search repository
func NewSearchRepository(db *database.DB) SearchRepository {
	return &searchRepository{db: db}
}

// UpsertDocument inserts or updates a single document in the search index
func (r *searchRepository) UpsertDocument(doc *models.SearchDocument) error {
	result := r.db.Exec(`
		INSERT INTO search_documents (entity_type, entity_id, title, content, metadata, accessible_by, is_public, creator_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?::jsonb, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (entity_type, entity_id)
		DO UPDATE SET
			title = EXCLUDED.title,
			content = EXCLUDED.content,
			metadata = EXCLUDED.metadata,
			accessible_by = EXCLUDED.accessible_by,
			is_public = EXCLUDED.is_public,
			creator_id = EXCLUDED.creator_id,
			updated_at = NOW()
	`, doc.EntityType, doc.EntityID, doc.Title, doc.Content,
		doc.Metadata, doc.AccessibleBy, doc.IsPublic, doc.CreatorID)

	if result.Error != nil {
		return fmt.Errorf("failed to upsert document: %w", result.Error)
	}
	return nil
}

// UpsertDocuments inserts or updates multiple documents
func (r *searchRepository) UpsertDocuments(docs []*models.SearchDocument) error {
	tx := r.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	for _, doc := range docs {
		result := tx.Exec(`
			INSERT INTO search_documents (entity_type, entity_id, title, content, metadata, accessible_by, is_public, creator_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?::jsonb, ?, ?, ?, NOW(), NOW())
			ON CONFLICT (entity_type, entity_id)
			DO UPDATE SET
				title = EXCLUDED.title,
				content = EXCLUDED.content,
				metadata = EXCLUDED.metadata,
				accessible_by = EXCLUDED.accessible_by,
				is_public = EXCLUDED.is_public,
				creator_id = EXCLUDED.creator_id,
				updated_at = NOW()
		`, doc.EntityType, doc.EntityID, doc.Title, doc.Content,
			doc.Metadata, doc.AccessibleBy, doc.IsPublic, doc.CreatorID)

		if result.Error != nil {
			tx.Rollback()
			return fmt.Errorf("failed to upsert document (type=%s, id=%d): %w", doc.EntityType, doc.EntityID, result.Error)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// DeleteDocument removes a document from the search index
func (r *searchRepository) DeleteDocument(entityType models.EntityType, entityID uint) error {
	result := r.db.Exec(
		"DELETE FROM search_documents WHERE entity_type = ? AND entity_id = ?",
		entityType, entityID,
	)
	if result.Error != nil {
		return fmt.Errorf("failed to delete document: %w", result.Error)
	}
	return nil
}

// Search performs a full-text search with permission filtering
// NOTE: cyrillic_lower() is used because lc_ctype=C does not lowercase Cyrillic
func (r *searchRepository) Search(query string, userID uint, userRole string, entityType string, limit, offset int) ([]models.SearchResult, int64, error) {
	// Build permission clause
	permissionClause := buildPermissionClause(userRole)

	// Build type filter
	typeFilter := ""
	if entityType != "" {
		typeFilter = "AND entity_type = @entity_type"
	}

	// Count query
	countSQL := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM search_documents
		WHERE (
			search_vector @@ (plainto_tsquery('russian', cyrillic_lower(@query)) || plainto_tsquery('english', cyrillic_lower(@query)))
			OR similarity(cyrillic_lower(title), cyrillic_lower(@query)) > 0.1
			OR similarity(cyrillic_lower(content), cyrillic_lower(@query)) > 0.05
		)
		AND %s
		%s
	`, permissionClause, typeFilter)

	var total int64
	countParams := map[string]interface{}{
		"query":   query,
		"user_id": userID,
	}
	if entityType != "" {
		countParams["entity_type"] = entityType
	}

	if err := r.db.Raw(countSQL, countParams).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	if total == 0 {
		return []models.SearchResult{}, 0, nil
	}

	// Search query with ranking
	searchSQL := fmt.Sprintf(`
		SELECT
			entity_type, entity_id, title,
			COALESCE(
				ts_headline('russian', content, plainto_tsquery('russian', cyrillic_lower(@query)),
					'StartSel=<mark>, StopSel=</mark>, MaxWords=30, MinWords=15, MaxFragments=2'),
				content
			) as content,
			metadata, created_at, updated_at,
			(
				COALESCE(ts_rank(search_vector, plainto_tsquery('russian', cyrillic_lower(@query))), 0) +
				COALESCE(ts_rank(search_vector, plainto_tsquery('english', cyrillic_lower(@query))), 0)
			) * 2.0 +
			COALESCE(similarity(cyrillic_lower(title), cyrillic_lower(@query)), 0) * 1.5 +
			COALESCE(similarity(cyrillic_lower(content), cyrillic_lower(@query)), 0) * 0.5
			AS rank
		FROM search_documents
		WHERE (
			search_vector @@ (plainto_tsquery('russian', cyrillic_lower(@query)) || plainto_tsquery('english', cyrillic_lower(@query)))
			OR similarity(cyrillic_lower(title), cyrillic_lower(@query)) > 0.1
			OR similarity(cyrillic_lower(content), cyrillic_lower(@query)) > 0.05
		)
		AND %s
		%s
		ORDER BY rank DESC, updated_at DESC
		LIMIT @limit OFFSET @offset
	`, permissionClause, typeFilter)

	searchParams := map[string]interface{}{
		"query":   query,
		"user_id": userID,
		"limit":   limit,
		"offset":  offset,
	}
	if entityType != "" {
		searchParams["entity_type"] = entityType
	}

	var results []models.SearchResult
	if err := r.db.Raw(searchSQL, searchParams).Scan(&results).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to search: %w", err)
	}

	// Parse metadata from JSON
	r.populateMetadata(&results)

	return results, total, nil
}

// CountByCategories returns the count of matching documents per entity type
// NOTE: cyrillic_lower() is used because lc_ctype=C does not lowercase Cyrillic
func (r *searchRepository) CountByCategories(query string, userID uint, userRole string, types []models.EntityType) (map[models.EntityType]int64, error) {
	permissionClause := buildPermissionClause(userRole)

	typeFilter := ""
	if len(types) > 0 {
		typeStrs := make([]string, len(types))
		for i, t := range types {
			typeStrs[i] = string(t)
		}
		typeFilter = fmt.Sprintf("AND entity_type IN ('%s')", strings.Join(typeStrs, "','"))
	}

	sql := fmt.Sprintf(`
		SELECT entity_type, COUNT(*) as cnt
		FROM search_documents
		WHERE (
			search_vector @@ (plainto_tsquery('russian', cyrillic_lower(@query)) || plainto_tsquery('english', cyrillic_lower(@query)))
			OR similarity(cyrillic_lower(title), cyrillic_lower(@query)) > 0.1
			OR similarity(cyrillic_lower(content), cyrillic_lower(@query)) > 0.05
		)
		AND %s
		%s
		GROUP BY entity_type
		ORDER BY cnt DESC
	`, permissionClause, typeFilter)

	type categoryCount struct {
		EntityType models.EntityType `gorm:"column:entity_type"`
		Cnt        int64             `gorm:"column:cnt"`
	}

	var counts []categoryCount
	if err := r.db.Raw(sql, map[string]interface{}{
		"query":   query,
		"user_id": userID,
	}).Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("failed to count by categories: %w", err)
	}

	result := make(map[models.EntityType]int64)
	for _, c := range counts {
		result[c.EntityType] = c.Cnt
	}

	logger.WithFields(map[string]interface{}{
		"query":            query,
		"user_id":          userID,
		"category_counts":  result,
		"total_categories": len(result),
	}).Info("[Search] CountByCategories result")

	return result, nil
}

// buildPermissionClause returns the SQL permission filter
func buildPermissionClause(userRole string) string {
	// Admins and super_admins bypass permission filtering
	if userRole == "admin" || userRole == "super_admin" {
		return "1=1"
	}
	return "(is_public = true OR creator_id = @user_id OR @user_id = ANY(accessible_by))"
}

// populateMetadata parses JSONB metadata for search results
func (r *searchRepository) populateMetadata(results *[]models.SearchResult) {
	// GORM should auto-scan JSONB into map[string]interface{} via the Scan method,
	// but we ensure metadata is never nil
	for i := range *results {
		if (*results)[i].Metadata == nil {
			(*results)[i].Metadata = make(map[string]interface{})
		}
	}
}

// ToInt64Array converts []uint to models.Int64Array
func ToInt64Array(ids []uint) models.Int64Array {
	arr := make(models.Int64Array, len(ids))
	for i, id := range ids {
		arr[i] = int64(id)
	}
	return arr
}
