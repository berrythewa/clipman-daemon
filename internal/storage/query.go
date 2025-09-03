package storage

import (
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// QueryBuilder provides a simple interface for building storage queries
type QueryBuilder struct {
	options QueryOptions
}

// SortOrder defines how results should be sorted
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// SortField defines what field to sort by
type SortField string

const (
	SortByCreated    SortField = "created"
	SortByLastSeen   SortField = "last_seen"
	SortBySize       SortField = "size"
	SortByType       SortField = "type"
	SortById         SortField = "id"
	SortByOccurrence SortField = "occurrences"
)

// QueryOptions defines filtering and sorting options for queries
type QueryOptions struct {
	// Time filters
	Before time.Time
	After  time.Time
	Since  time.Time // Alias for After

	// Content filters
	ContentType  types.ContentType
	ContentTypes []types.ContentType
	MinSize      int64
	MaxSize      int64
	Tags         []string

	// ID filters
	IDs       []int64
	Hashes    []string
	DeviceIDs []string

	// Text search
	Contains      string
	Regex         string // Regex pattern matching
	CaseSensitive bool

	// Result control
	Limit     int64
	Offset    int64
	SortBy    SortField
	SortOrder SortOrder

	// Basic filtering
	HasOccurrences *bool
	MinOccurrences int
	MaxOccurrences int
	CompressedOnly *bool
}

// NewQuery creates a new QueryBuilder
func NewQuery() *QueryBuilder {
	return &QueryBuilder{
		options: QueryOptions{
			SortOrder: SortDesc, // Default: newest first
			SortBy:    SortByCreated,
		},
	}
}

// Time-based filtering
func (qb *QueryBuilder) CreatedAfter(t time.Time) *QueryBuilder {
	qb.options.After = t
	return qb
}

func (qb *QueryBuilder) CreatedBefore(t time.Time) *QueryBuilder {
	qb.options.Before = t
	return qb
}

func (qb *QueryBuilder) Since(t time.Time) *QueryBuilder {
	return qb.CreatedAfter(t)
}

// Content-based filtering
func (qb *QueryBuilder) OfType(contentType types.ContentType) *QueryBuilder {
	qb.options.ContentType = contentType
	return qb
}

func (qb *QueryBuilder) MinSize(size int64) *QueryBuilder {
	qb.options.MinSize = size
	return qb
}

func (qb *QueryBuilder) MaxSize(size int64) *QueryBuilder {
	qb.options.MaxSize = size
	return qb
}

// Text search filtering
func (qb *QueryBuilder) Contains(text string) *QueryBuilder {
	qb.options.Contains = text
	return qb
}

// ID-based filtering
func (qb *QueryBuilder) WithIDs(ids ...int64) *QueryBuilder {
	qb.options.IDs = ids
	return qb
}

func (qb *QueryBuilder) WithHashes(hashes ...string) *QueryBuilder {
	qb.options.Hashes = hashes
	return qb
}

// Tag-based filtering
func (qb *QueryBuilder) WithTags(tags ...string) *QueryBuilder {
	qb.options.Tags = tags
	return qb
}

// Occurrence filtering
func (qb *QueryBuilder) WithOccurrences() *QueryBuilder {
	hasOccurrences := true
	qb.options.HasOccurrences = &hasOccurrences
	return qb
}

// Sorting
func (qb *QueryBuilder) OrderByCreated() *QueryBuilder {
	qb.options.SortBy = SortByCreated
	qb.options.SortOrder = SortDesc
	return qb
}

func (qb *QueryBuilder) OrderBySize() *QueryBuilder {
	qb.options.SortBy = SortBySize
	qb.options.SortOrder = SortDesc
	return qb
}

func (qb *QueryBuilder) OrderByOccurrences() *QueryBuilder {
	qb.options.SortBy = SortByOccurrence
	qb.options.SortOrder = SortDesc
	return qb
}

// Limit and pagination
func (qb *QueryBuilder) Limit(limit int64) *QueryBuilder {
	qb.options.Limit = limit
	return qb
}

func (qb *QueryBuilder) Offset(offset int64) *QueryBuilder {
	qb.options.Offset = offset
	return qb
}

// Execute runs the query against the provided storage
func (qb *QueryBuilder) Execute(storage Storage) ([]*types.ClipboardContent, error) {
	return storage.Query(qb.options)
}
