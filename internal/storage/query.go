package storage

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// QueryBuilder provides a SQL-like interface for building storage queries
type QueryBuilder struct {
	options QueryOptions
	errors  []string
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
	SortByLastSeen   SortField = "last_seen" // Latest occurrence
	SortBySize       SortField = "size"
	SortByType       SortField = "type"
	SortById         SortField = "id"
	SortByOccurrence SortField = "occurrences" // Count of occurrences
)

// AggregateFunction defines aggregation operations
type AggregateFunction string

const (
	AggCount   AggregateFunction = "COUNT"
	AggSum     AggregateFunction = "SUM"
	AggAvg     AggregateFunction = "AVG"
	AggMin     AggregateFunction = "MIN"
	AggMax     AggregateFunction = "MAX"
	AggFirst   AggregateFunction = "FIRST"
	AggLast    AggregateFunction = "LAST"
)

// AggregateOptions defines what to aggregate and how
type AggregateOptions struct {
	Function AggregateFunction
	Field    string // Field to aggregate on ("size", "occurrences", etc.)
	GroupBy  []string // Fields to group by ("type", "device_id", etc.)
}

// QueryOptions now includes more advanced filtering (enhanced from boltdb.go)
type QueryOptions struct {
	// Time filters
	Before      time.Time
	After       time.Time
	Since       time.Time // Alias for After

	// Content filters
	ContentType  types.ContentType
	ContentTypes []types.ContentType // Multiple types
	MinSize      int64
	MaxSize      int64

	// ID filters
	IDs        []int64
	Hashes     []string
	DeviceIDs  []string

	// Text search
	Contains    string // Search within content data
	Regex       string // Regex pattern matching
	CaseSensitive bool

	// Result control
	Limit    int64
	Offset   int64
	SortBy   SortField
	SortOrder SortOrder

	// Advanced filtering
	HasOccurrences  *bool // Filter by whether item has multiple occurrences
	MinOccurrences  int   // Minimum occurrence count
	MaxOccurrences  int   // Maximum occurrence count
	CompressedOnly  *bool // Filter by compression status

	// Aggregation
	Aggregate *AggregateOptions
}

// AggregateResult holds results of aggregation queries
type AggregateResult struct {
	Function AggregateFunction
	Field    string
	Value    interface{} // int64, float64, string, time.Time, etc.
	GroupBy  map[string]interface{} // Group-by values
	Count    int64 // Number of items in this group
}

// NewQuery creates a new QueryBuilder (SQL-like interface)
func NewQuery() *QueryBuilder {
	return &QueryBuilder{
		options: QueryOptions{
			SortOrder: SortDesc, // Default: newest first
			SortBy:    SortByCreated,
		},
	}
}

// SELECT equivalent - what to retrieve
func (qb *QueryBuilder) Select() *QueryBuilder {
	return qb // For fluent interface
}

// FROM equivalent - not needed for single-table storage but kept for SQL-like syntax
func (qb *QueryBuilder) From() *QueryBuilder {
	return qb
}

// WHERE clauses for filtering
func (qb *QueryBuilder) Where() *QueryBuilder {
	return qb
}

// Time-based filtering (like SQL WHERE clauses)
func (qb *QueryBuilder) CreatedAfter(t time.Time) *QueryBuilder {
	qb.options.After = t
	return qb
}

func (qb *QueryBuilder) CreatedBefore(t time.Time) *QueryBuilder {
	qb.options.Before = t
	return qb
}

func (qb *QueryBuilder) CreatedBetween(start, end time.Time) *QueryBuilder {
	qb.options.After = start
	qb.options.Before = end
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

func (qb *QueryBuilder) OfTypes(contentTypes ...types.ContentType) *QueryBuilder {
	qb.options.ContentTypes = contentTypes
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

func (qb *QueryBuilder) SizeBetween(min, max int64) *QueryBuilder {
	return qb.MinSize(min).MaxSize(max)
}

// Text search filtering
func (qb *QueryBuilder) Contains(text string) *QueryBuilder {
	qb.options.Contains = text
	return qb
}

func (qb *QueryBuilder) ContainsCaseSensitive(text string) *QueryBuilder {
	qb.options.Contains = text
	qb.options.CaseSensitive = true
	return qb
}

func (qb *QueryBuilder) MatchesRegex(pattern string) *QueryBuilder {
	qb.options.Regex = pattern
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

func (qb *QueryBuilder) FromDevices(deviceIDs ...string) *QueryBuilder {
	qb.options.DeviceIDs = deviceIDs
	return qb
}

// Occurrence-based filtering
func (qb *QueryBuilder) WithOccurrences() *QueryBuilder {
	hasOccurrences := true
	qb.options.HasOccurrences = &hasOccurrences
	return qb
}

func (qb *QueryBuilder) WithoutOccurrences() *QueryBuilder {
	hasOccurrences := false
	qb.options.HasOccurrences = &hasOccurrences
	return qb
}

func (qb *QueryBuilder) MinOccurrences(count int) *QueryBuilder {
	qb.options.MinOccurrences = count
	return qb
}

func (qb *QueryBuilder) MaxOccurrences(count int) *QueryBuilder {
	qb.options.MaxOccurrences = count
	return qb
}

// Compression filtering
func (qb *QueryBuilder) CompressedOnly() *QueryBuilder {
	compressed := true
	qb.options.CompressedOnly = &compressed
	return qb
}

func (qb *QueryBuilder) UncompressedOnly() *QueryBuilder {
	compressed := false
	qb.options.CompressedOnly = &compressed
	return qb
}

// ORDER BY equivalent
func (qb *QueryBuilder) OrderBy(field SortField, order SortOrder) *QueryBuilder {
	qb.options.SortBy = field
	qb.options.SortOrder = order
	return qb
}

func (qb *QueryBuilder) OrderByCreated() *QueryBuilder {
	return qb.OrderBy(SortByCreated, SortDesc)
}

func (qb *QueryBuilder) OrderByCreatedAsc() *QueryBuilder {
	return qb.OrderBy(SortByCreated, SortAsc)
}

func (qb *QueryBuilder) OrderBySize() *QueryBuilder {
	return qb.OrderBy(SortBySize, SortDesc)
}

func (qb *QueryBuilder) OrderByOccurrences() *QueryBuilder {
	return qb.OrderBy(SortByOccurrence, SortDesc)
}

// LIMIT and OFFSET equivalent
func (qb *QueryBuilder) Limit(limit int64) *QueryBuilder {
	qb.options.Limit = limit
	return qb
}

func (qb *QueryBuilder) Offset(offset int64) *QueryBuilder {
	qb.options.Offset = offset
	return qb
}

// Pagination helper
func (qb *QueryBuilder) Paginate(page, pageSize int64) *QueryBuilder {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize
	return qb.Limit(pageSize).Offset(offset)
}

// Aggregation functions (like SQL COUNT, SUM, etc.)
func (qb *QueryBuilder) Count() *QueryBuilder {
	qb.options.Aggregate = &AggregateOptions{
		Function: AggCount,
		Field:    "*",
	}
	return qb
}

func (qb *QueryBuilder) Sum(field string) *QueryBuilder {
	qb.options.Aggregate = &AggregateOptions{
		Function: AggSum,
		Field:    field,
	}
	return qb
}

func (qb *QueryBuilder) Avg(field string) *QueryBuilder {
	qb.options.Aggregate = &AggregateOptions{
		Function: AggAvg,
		Field:    field,
	}
	return qb
}

func (qb *QueryBuilder) Min(field string) *QueryBuilder {
	qb.options.Aggregate = &AggregateOptions{
		Function: AggMin,
		Field:    field,
	}
	return qb
}

func (qb *QueryBuilder) Max(field string) *QueryBuilder {
	qb.options.Aggregate = &AggregateOptions{
		Function: AggMax,
		Field:    field,
	}
	return qb
}

func (qb *QueryBuilder) First() *QueryBuilder {
	qb.options.Aggregate = &AggregateOptions{
		Function: AggFirst,
		Field:    "*",
	}
	return qb
}

func (qb *QueryBuilder) Last() *QueryBuilder {
	qb.options.Aggregate = &AggregateOptions{
		Function: AggLast,
		Field:    "*",
	}
	return qb
}

// GROUP BY equivalent
func (qb *QueryBuilder) GroupBy(fields ...string) *QueryBuilder {
	if qb.options.Aggregate == nil {
		qb.addError("GroupBy requires an aggregate function (Count, Sum, Avg, etc.)")
		return qb
	}
	qb.options.Aggregate.GroupBy = fields
	return qb
}

// Validation and execution
func (qb *QueryBuilder) addError(err string) {
	qb.errors = append(qb.errors, err)
}

func (qb *QueryBuilder) Validate() error {
	if len(qb.errors) > 0 {
		return fmt.Errorf("query validation failed: %s", strings.Join(qb.errors, "; "))
	}
	return nil
}

// Build returns the final QueryOptions
func (qb *QueryBuilder) Build() (QueryOptions, error) {
	if err := qb.Validate(); err != nil {
		return QueryOptions{}, err
	}
	return qb.options, nil
}

// Execute runs the query against the provided storage
func (qb *QueryBuilder) Execute(storage IStorage) ([]*types.ClipboardContent, error) {
	options, err := qb.Build()
	if err != nil {
		return nil, err
	}
	return storage.Query(options)
}

// ExecuteAggregate runs an aggregation query
func (qb *QueryBuilder) ExecuteAggregate(storage IStorage) ([]AggregateResult, error) {
	options, err := qb.Build()
	if err != nil {
		return nil, err
	}
	if options.Aggregate == nil {
		return nil, fmt.Errorf("no aggregate function specified")
	}

	// Check if storage supports aggregation
	if aggregator, ok := storage.(IAggregateStorage); ok {
		return aggregator.Aggregate(options)
	}

	// Fallback: execute normal query and aggregate in memory
	return qb.executeAggregateInMemory(storage, options)
}

// executeAggregateInMemory performs aggregation in memory for storages that don't support native aggregation
func (qb *QueryBuilder) executeAggregateInMemory(storage IStorage, options QueryOptions) ([]AggregateResult, error) {
	// Remove aggregation from options for the base query
	baseOptions := options
	baseOptions.Aggregate = nil

	contents, err := storage.Query(baseOptions)
	if err != nil {
		return nil, err
	}

	return qb.aggregateInMemory(contents, options.Aggregate), nil
}

// aggregateInMemory performs in-memory aggregation
func (qb *QueryBuilder) aggregateInMemory(contents []*types.ClipboardContent, agg *AggregateOptions) []AggregateResult {
	if agg == nil {
		return nil
	}

	// Simple aggregation without grouping
	if len(agg.GroupBy) == 0 {
		return []AggregateResult{qb.calculateAggregate(contents, agg, "")}
	}

	// Group the contents
	grouped := qb.groupContents(contents, agg.GroupBy)
	
	// Calculate aggregates for each group
	results := make([]AggregateResult, 0, len(grouped))
	for groupKey, groupContents := range grouped {
		result := qb.calculateAggregate(groupContents, agg, groupKey)
		results = append(results, result)
	}

	return results
}

// groupContents groups content by specified fields
func (qb *QueryBuilder) groupContents(contents []*types.ClipboardContent, groupBy []string) map[string][]*types.ClipboardContent {
	groups := make(map[string][]*types.ClipboardContent)
	
	for _, content := range contents {
		key := qb.buildGroupKey(content, groupBy)
		groups[key] = append(groups[key], content)
	}
	
	return groups
}

// buildGroupKey creates a group key based on specified fields
func (qb *QueryBuilder) buildGroupKey(content *types.ClipboardContent, fields []string) string {
	var keyParts []string
	
	for _, field := range fields {
		switch field {
		case "type":
			keyParts = append(keyParts, string(content.Type))
		case "device_id":
			keyParts = append(keyParts, content.DeviceID)
		case "compressed":
			if content.Compressed {
				keyParts = append(keyParts, "compressed")
			} else {
				keyParts = append(keyParts, "uncompressed")
			}
		default:
			keyParts = append(keyParts, "unknown")
		}
	}
	
	return strings.Join(keyParts, "|")
}

// calculateAggregate performs the actual aggregation calculation
func (qb *QueryBuilder) calculateAggregate(contents []*types.ClipboardContent, agg *AggregateOptions, groupKey string) AggregateResult {
	result := AggregateResult{
		Function: agg.Function,
		Field:    agg.Field,
		Count:    int64(len(contents)),
	}

	// Parse group key back into map
	if groupKey != "" && len(agg.GroupBy) > 0 {
		result.GroupBy = make(map[string]interface{})
		parts := strings.Split(groupKey, "|")
		for i, field := range agg.GroupBy {
			if i < len(parts) {
				result.GroupBy[field] = parts[i]
			}
		}
	}

	switch agg.Function {
	case AggCount:
		result.Value = result.Count
	
	case AggSum:
		var sum int64
		for _, content := range contents {
			switch agg.Field {
			case "size":
				sum += int64(len(content.Data))
			case "occurrences":
				sum += int64(len(content.Occurrences))
			}
		}
		result.Value = sum
	
	case AggAvg:
		if result.Count == 0 {
			result.Value = 0.0
		} else {
			var sum int64
			for _, content := range contents {
				switch agg.Field {
				case "size":
					sum += int64(len(content.Data))
				case "occurrences":
					sum += int64(len(content.Occurrences))
				}
			}
			result.Value = float64(sum) / float64(result.Count)
		}
	
	case AggMin:
		result.Value = qb.findMinValue(contents, agg.Field)
	
	case AggMax:
		result.Value = qb.findMaxValue(contents, agg.Field)
	
	case AggFirst:
		if len(contents) > 0 {
			// Sort by creation time and take first
			sort.Slice(contents, func(i, j int) bool {
				return contents[i].Created.Before(contents[j].Created)
			})
			result.Value = contents[0]
		}
	
	case AggLast:
		if len(contents) > 0 {
			// Sort by creation time and take last
			sort.Slice(contents, func(i, j int) bool {
				return contents[i].Created.Before(contents[j].Created)
			})
			result.Value = contents[len(contents)-1]
		}
	}

	return result
}

// findMinValue finds minimum value for a specific field
func (qb *QueryBuilder) findMinValue(contents []*types.ClipboardContent, field string) interface{} {
	if len(contents) == 0 {
		return nil
	}

	switch field {
	case "size":
		min := int64(len(contents[0].Data))
		for _, content := range contents[1:] {
			size := int64(len(content.Data))
			if size < min {
				min = size
			}
		}
		return min
		
	case "created":
		min := contents[0].Created
		for _, content := range contents[1:] {
			if content.Created.Before(min) {
				min = content.Created
			}
		}
		return min
		
	case "occurrences":
		min := int64(len(contents[0].Occurrences))
		for _, content := range contents[1:] {
			count := int64(len(content.Occurrences))
			if count < min {
				min = count
			}
		}
		return min
	}
	
	return nil
}

// findMaxValue finds maximum value for a specific field  
func (qb *QueryBuilder) findMaxValue(contents []*types.ClipboardContent, field string) interface{} {
	if len(contents) == 0 {
		return nil
	}

	switch field {
	case "size":
		max := int64(len(contents[0].Data))
		for _, content := range contents[1:] {
			size := int64(len(content.Data))
			if size > max {
				max = size
			}
		}
		return max
		
	case "created":
		max := contents[0].Created
		for _, content := range contents[1:] {
			if content.Created.After(max) {
				max = content.Created
			}
		}
		return max
		
	case "occurrences":
		max := int64(len(contents[0].Occurrences))
		for _, content := range contents[1:] {
			count := int64(len(content.Occurrences))
			if count > max {
				max = count
			}
		}
		return max
	}
	
	return nil
}
