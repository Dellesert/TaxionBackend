package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/search/models"
	"tachyon-messenger/services/search/repository"
	"tachyon-messenger/shared/logger"
	sharedredis "tachyon-messenger/shared/redis"
)

// SearchUsecase defines the interface for search business logic
type SearchUsecase interface {
	// Search
	Search(query string, userID uint, userRole string, req *models.SearchRequest) (*models.SearchResponse, error)

	// Indexing (called by other services via internal API)
	IndexDocument(req *models.IndexDocumentRequest) error
	BulkIndexDocuments(req *models.BulkIndexRequest) error
	DeleteDocument(entityType models.EntityType, entityID uint) error
}

type searchUsecase struct {
	searchRepo  repository.SearchRepository
	redisClient *sharedredis.Client
}

// NewSearchUsecase creates a new search usecase
func NewSearchUsecase(searchRepo repository.SearchRepository, redisClient *sharedredis.Client) SearchUsecase {
	return &searchUsecase{
		searchRepo:  searchRepo,
		redisClient: redisClient,
	}
}

// Search performs a global search with categorized results
func (uc *searchUsecase) Search(query string, userID uint, userRole string, req *models.SearchRequest) (*models.SearchResponse, error) {
	// Normalize query
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	// Set default limit
	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	// Try cache first
	cacheKey := uc.buildCacheKey(userID, query, req)
	if cached := uc.getFromCache(cacheKey); cached != nil {
		return cached, nil
	}

	var response *models.SearchResponse

	// If specific category requested - paginate only that category
	if req.Category != "" {
		entityType := models.EntityType(req.Category)
		if !entityType.IsValid() {
			return nil, fmt.Errorf("invalid category: %s", req.Category)
		}

		results, total, err := uc.searchRepo.Search(query, userID, userRole, string(entityType), limit, req.Offset)
		if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}

		response = &models.SearchResponse{
			Query: query,
			Categories: []models.CategoryResults{
				{
					Type:    entityType,
					Results: results,
					Total:   total,
					HasMore: int64(req.Offset+limit) < total,
				},
			},
			TotalCount: total,
		}
	} else {
		// Initial search - get counts per category first
		var types []models.EntityType
		if len(req.Types) > 0 {
			types = req.Types
		}

		categoryCounts, err := uc.searchRepo.CountByCategories(query, userID, userRole, types)
		if err != nil {
			return nil, fmt.Errorf("failed to count by categories: %w", err)
		}

		// Fetch first N results per category
		var categories []models.CategoryResults
		var totalCount int64

		// Define order of categories
		orderedTypes := models.AllEntityTypes()
		if len(types) > 0 {
			orderedTypes = types
		}

		for _, entityType := range orderedTypes {
			count, exists := categoryCounts[entityType]
			if !exists || count == 0 {
				continue
			}

			results, _, err := uc.searchRepo.Search(query, userID, userRole, string(entityType), limit, 0)
			if err != nil {
				logger.WithFields(map[string]interface{}{
					"entity_type": entityType,
					"error":       err.Error(),
				}).Warn("Failed to search category, skipping")
				continue
			}

			categories = append(categories, models.CategoryResults{
				Type:    entityType,
				Results: results,
				Total:   count,
				HasMore: count > int64(limit),
			})
			totalCount += count
		}

		response = &models.SearchResponse{
			Query:      query,
			Categories: categories,
			TotalCount: totalCount,
		}
	}

	// Cache the result
	uc.setCache(cacheKey, response)

	return response, nil
}

// IndexDocument indexes or updates a single document
func (uc *searchUsecase) IndexDocument(req *models.IndexDocumentRequest) error {
	if !req.EntityType.IsValid() {
		return fmt.Errorf("invalid entity type: %s", req.EntityType)
	}

	doc := &models.SearchDocument{
		EntityType:   req.EntityType,
		EntityID:     req.EntityID,
		Title:        req.Title,
		Content:      req.Content,
		Metadata:     models.JSONB(req.Metadata),
		AccessibleBy: toInt64Array(req.AccessibleBy),
		IsPublic:     req.IsPublic,
		CreatorID:    req.CreatorID,
	}

	if err := uc.searchRepo.UpsertDocument(doc); err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}

	// Invalidate cache
	uc.invalidateCache()

	return nil
}

// BulkIndexDocuments indexes multiple documents at once
func (uc *searchUsecase) BulkIndexDocuments(req *models.BulkIndexRequest) error {
	docs := make([]*models.SearchDocument, 0, len(req.Documents))

	for _, docReq := range req.Documents {
		if !docReq.EntityType.IsValid() {
			return fmt.Errorf("invalid entity type: %s", docReq.EntityType)
		}

		docs = append(docs, &models.SearchDocument{
			EntityType:   docReq.EntityType,
			EntityID:     docReq.EntityID,
			Title:        docReq.Title,
			Content:      docReq.Content,
			Metadata:     models.JSONB(docReq.Metadata),
			AccessibleBy: toInt64Array(docReq.AccessibleBy),
			IsPublic:     docReq.IsPublic,
			CreatorID:    docReq.CreatorID,
		})
	}

	if err := uc.searchRepo.UpsertDocuments(docs); err != nil {
		return fmt.Errorf("failed to bulk index documents: %w", err)
	}

	// Invalidate cache
	uc.invalidateCache()

	return nil
}

// DeleteDocument removes a document from the search index
func (uc *searchUsecase) DeleteDocument(entityType models.EntityType, entityID uint) error {
	if !entityType.IsValid() {
		return fmt.Errorf("invalid entity type: %s", entityType)
	}

	if err := uc.searchRepo.DeleteDocument(entityType, entityID); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	// Invalidate cache
	uc.invalidateCache()

	return nil
}

// ----- Cache helpers -----

const cacheTTL = 60 * time.Second
const cachePrefix = "search:results:"

func (uc *searchUsecase) buildCacheKey(userID uint, query string, req *models.SearchRequest) string {
	return fmt.Sprintf("%s%d:%s:%s:%d:%d", cachePrefix, userID, query, req.Category, req.Limit, req.Offset)
}

func (uc *searchUsecase) getFromCache(key string) *models.SearchResponse {
	if uc.redisClient == nil {
		return nil
	}

	data, err := uc.redisClient.Get(key)
	if err != nil {
		return nil
	}

	var response models.SearchResponse
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		return nil
	}

	return &response
}

func (uc *searchUsecase) setCache(key string, response *models.SearchResponse) {
	if uc.redisClient == nil {
		return
	}

	data, err := json.Marshal(response)
	if err != nil {
		return
	}

	ctx := context.Background()
	uc.redisClient.Client.Set(ctx, key, string(data), cacheTTL)
}

func (uc *searchUsecase) invalidateCache() {
	if uc.redisClient == nil {
		return
	}

	ctx := context.Background()
	// Delete all search result cache keys
	iter := uc.redisClient.Client.Scan(ctx, 0, cachePrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		uc.redisClient.Client.Del(ctx, iter.Val())
	}
}

// toInt64Array converts []uint to models.Int64Array
func toInt64Array(ids []uint) models.Int64Array {
	arr := make(models.Int64Array, len(ids))
	for i, id := range ids {
		arr[i] = int64(id)
	}
	return arr
}
