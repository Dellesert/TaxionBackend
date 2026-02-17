package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Int64Array represents a PostgreSQL bigint[] array
type Int64Array []int64

// Value implements driver.Valuer for Int64Array
func (a Int64Array) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	parts := make([]string, len(a))
	for i, v := range a {
		parts[i] = strconv.FormatInt(v, 10)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

// Scan implements sql.Scanner for Int64Array
func (a *Int64Array) Scan(src interface{}) error {
	if src == nil {
		*a = Int64Array{}
		return nil
	}
	var s string
	switch v := src.(type) {
	case []byte:
		s = string(v)
	case string:
		s = v
	default:
		return fmt.Errorf("cannot scan %T into Int64Array", src)
	}
	s = strings.Trim(s, "{}")
	if s == "" {
		*a = Int64Array{}
		return nil
	}
	parts := strings.Split(s, ",")
	result := make(Int64Array, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as int64: %w", p, err)
		}
		result[i] = v
	}
	*a = result
	return nil
}

// EntityType represents the type of searchable entity
type EntityType string

const (
	EntityTypeTask     EntityType = "task"
	EntityTypePoll     EntityType = "poll"
	EntityTypeChat     EntityType = "chat"
	EntityTypeMessage  EntityType = "message"
	EntityTypeSchedule EntityType = "schedule"
	EntityTypeEvent    EntityType = "event"
)

// AllEntityTypes returns all valid entity types
func AllEntityTypes() []EntityType {
	return []EntityType{
		EntityTypeTask,
		EntityTypePoll,
		EntityTypeChat,
		EntityTypeMessage,
		EntityTypeSchedule,
		EntityTypeEvent,
	}
}

// IsValid checks if an entity type is valid
func (e EntityType) IsValid() bool {
	for _, t := range AllEntityTypes() {
		if e == t {
			return true
		}
	}
	return false
}

// JSONB is a wrapper for JSONB fields in PostgreSQL
type JSONB map[string]interface{}

// Value implements driver.Valuer for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = JSONB{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

// SearchDocument represents a document in the search index
type SearchDocument struct {
	ID           uint          `gorm:"primarykey" json:"id"`
	EntityType   EntityType    `gorm:"column:entity_type;not null;size:30" json:"entity_type"`
	EntityID     uint          `gorm:"column:entity_id;not null" json:"entity_id"`
	Title        string        `gorm:"column:title;type:text;not null;default:''" json:"title"`
	Content      string        `gorm:"column:content;type:text;not null;default:''" json:"content"`
	SearchVector string        `gorm:"column:search_vector;type:tsvector;not null" json:"-"`
	Metadata     JSONB         `gorm:"column:metadata;type:jsonb;not null;default:'{}'" json:"metadata"`
	AccessibleBy Int64Array `gorm:"column:accessible_by;type:bigint[];not null;default:'{}'" json:"-"`
	IsPublic     bool          `gorm:"column:is_public;not null;default:false" json:"-"`
	CreatorID    uint          `gorm:"column:creator_id;not null;default:0" json:"-"`
	CreatedAt    time.Time     `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt    time.Time     `gorm:"column:updated_at;not null" json:"updated_at"`
}

// TableName returns the table name
func (SearchDocument) TableName() string {
	return "search_documents"
}

// ----- Request Types -----

// IndexDocumentRequest is the request body for indexing a document
type IndexDocumentRequest struct {
	EntityType   EntityType             `json:"entity_type" binding:"required"`
	EntityID     uint                   `json:"entity_id" binding:"required,min=1"`
	Title        string                 `json:"title"`
	Content      string                 `json:"content"`
	Metadata     map[string]interface{} `json:"metadata"`
	AccessibleBy []uint                 `json:"accessible_by"`
	IsPublic     bool                   `json:"is_public"`
	CreatorID    uint                   `json:"creator_id"`
}

// BulkIndexRequest is the request body for bulk indexing
type BulkIndexRequest struct {
	Documents []IndexDocumentRequest `json:"documents" binding:"required,min=1"`
}

// DeleteDocumentRequest is the request body for deleting a document
type DeleteDocumentRequest struct {
	EntityType EntityType `json:"entity_type" binding:"required"`
	EntityID   uint       `json:"entity_id" binding:"required,min=1"`
}

// SearchRequest is the query parameters for a search request
type SearchRequest struct {
	Query    string       `form:"q" binding:"required,min=1,max=200"`
	Types    []EntityType `form:"type"`
	Limit    int          `form:"limit"`
	Offset   int          `form:"offset"`
	Category string       `form:"category"`
}

// ----- Response Types -----

// SearchResult represents a single search result item
type SearchResult struct {
	EntityType EntityType             `json:"entity_type"`
	EntityID   uint                   `json:"entity_id"`
	Title      string                 `json:"title"`
	Content    string                 `json:"content"`
	Metadata   map[string]interface{} `json:"metadata"`
	Rank       float64                `json:"rank"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// CategoryResults represents search results for a single category
type CategoryResults struct {
	Type    EntityType     `json:"type"`
	Results []SearchResult `json:"results"`
	Total   int64          `json:"total"`
	HasMore bool           `json:"has_more"`
}

// SearchResponse is the full search response with categorized results
type SearchResponse struct {
	Query      string            `json:"query"`
	Categories []CategoryResults `json:"categories"`
	TotalCount int64             `json:"total_count"`
}
