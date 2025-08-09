# Storage Package - Storage-Agnostic Query System

The storage package provides a **completely storage-agnostic** clipboard content management system with SQL-like query capabilities, similar to MongoDB queries but with strong typing and compile-time safety.

## 🎯 Key Features

### ✅ **Completed**
- **Storage Agnostic**: Works with any storage backend (BoltDB implemented, SQLite/PostgreSQL/MongoDB ready)
- **SQL-like Query Builder**: Fluent interface for complex queries
- **Advanced Filtering**: Time, content type, size, device, text search, occurrences, compression
- **Aggregation Support**: COUNT, SUM, AVG, MIN, MAX, FIRST, LAST with GROUP BY
- **Sorting & Pagination**: Efficient data retrieval with multiple sort options
- **Statistics**: Comprehensive storage statistics and analytics
- **Factory Pattern**: Easy storage backend switching
- **Type Safety**: Compile-time query validation
- **Performance Optimized**: ID/Hash lookups, size filtering, memory-efficient pagination

### 🚧 **Ready for Implementation**
- **Transaction Support**: Interface defined, ready for implementation
- **SQLite Backend**: Factory configured, awaiting implementation  
- **PostgreSQL Backend**: Interface ready
- **MongoDB Backend**: Interface ready
- **Regex Search**: Framework in place
- **Memory Storage**: For testing/development

## 📁 File Structure

```
internal/storage/
├── README.md           # This file - comprehensive documentation
├── factory.go          # Storage factory for backend switching
├── query.go           # SQL-like query builder with aggregation
├── boltdb.go          # BoltDB implementation with enhanced Query()
├── store.go           # Content storage operations 
├── delete.go          # Content deletion operations
├── stats.go           # Statistics and analytics
├── utils.go           # Content processing utilities
├── examples.go        # Usage examples and patterns
└── boltdb.old.go      # Legacy implementation (for reference)
```

## 🚀 Quick Start

### 1. Create Storage Instance

```go
// Using factory pattern (recommended)
factory := storage.NewStorageFactory(logger)
config := storage.DefaultStorageConfig(
    storage.StorageTypeBoltDB, 
    "clipboard.db", 
    logger, 
    "device-123",
)

store, err := factory.CreateStorage(config)
if err != nil {
    panic(err)
}
defer store.Close()
```

### 2. Basic Operations

```go
// Add content
content := &types.ClipboardContent{
    Type: types.TypeText,
    Data: []byte("Hello, World!"),
    Created: time.Now(),
    DeviceID: "device-123",
}
err := store.AddContent(content)

// Get latest items
latest, err := storage.NewQuery().
    OrderByCreated().
    Limit(10).
    Execute(store)
```

### 3. Advanced Queries (SQL-like)

```go
// Complex filtering
results, err := storage.NewQuery().
    Select().
    Where().
    OfType(types.TypeText).
    Contains("important").
    SizeBetween(100, 10000).
    CreatedAfter(time.Now().AddDate(0, 0, -7)). // Last 7 days
    WithOccurrences().                           // Has multiple uses
    OrderByOccurrences().
    Limit(50).
    Execute(store)

// Pagination
page1, err := storage.NewQuery().
    OrderByCreated().
    Paginate(1, 20). // Page 1, 20 items per page
    Execute(store)
```

### 4. Aggregation Queries

```go
// Count all items
totalCount, _ := storage.NewQuery().
    Count().
    ExecuteAggregate(store)

// Average size by content type
avgSizes, _ := storage.NewQuery().
    Avg("size").
    GroupBy("type").
    ExecuteAggregate(store)

// Most frequently used items
topUsed, _ := storage.NewQuery().
    Count().
    GroupBy("hash").
    ExecuteAggregate(store)
```

## 🎯 Query Builder API

### Selection & Basic Structure
```go
query := storage.NewQuery().
    Select().   // Optional, for SQL-like syntax
    From().     // Optional, for SQL-like syntax  
    Where()     // Optional, for SQL-like syntax
```

### Time-based Filtering
```go
.CreatedAfter(time.Now().AddDate(0, 0, -7))     // Last 7 days
.CreatedBefore(time.Now().AddDate(0, 0, -1))    // Before yesterday
.CreatedBetween(start, end)                      // Date range
.Since(time.Now().AddDate(0, 0, -30))           // Alias for CreatedAfter
```

### Content Filtering
```go
.OfType(types.TypeText)                          // Single type
.OfTypes(types.TypeText, types.TypeImage)        // Multiple types
.MinSize(1024)                                   // Min size in bytes
.MaxSize(1024*1024)                             // Max size in bytes  
.SizeBetween(1024, 1024*1024)                   // Size range
```

### Text Search
```go
.Contains("search term")                         // Case-insensitive search
.ContainsCaseSensitive("Search Term")           // Case-sensitive search
.MatchesRegex(`\d{4}-\d{2}-\d{2}`)              // Regex pattern (future)
```

### ID/Hash Filtering (Most Efficient)
```go
.WithIDs(1, 2, 3, 4, 5)                         // Specific IDs
.WithHashes("hash1", "hash2")                    // Specific hashes
.FromDevices("device-1", "device-2")            // Specific devices
```

### Occurrence-based Filtering  
```go
.WithOccurrences()                               // Has multiple occurrences
.WithoutOccurrences()                           // Single occurrence only
.MinOccurrences(3)                              // At least N occurrences
.MaxOccurrences(10)                             // At most N occurrences
```

### Compression Filtering
```go
.CompressedOnly()                               // Only compressed items
.UncompressedOnly()                             // Only uncompressed items
```

### Sorting
```go
.OrderBy(storage.SortByCreated, storage.SortDesc)  // Custom sorting
.OrderByCreated()                                   // Newest first (default)  
.OrderByCreatedAsc()                               // Oldest first
.OrderBySize()                                      // Largest first
.OrderByOccurrences()                              // Most used first
```

### Pagination & Limits
```go
.Limit(100)                                     // Max results
.Offset(50)                                     // Skip first N results
.Paginate(2, 25)                               // Page 2, 25 per page
```

### Aggregation Functions
```go
.Count()                                        // COUNT(*)
.Sum("size")                                    // SUM(size)
.Avg("size")                                    // AVG(size)  
.Min("created")                                 // MIN(created)
.Max("created")                                 // MAX(created)
.First()                                        // Oldest item
.Last()                                         // Newest item
.GroupBy("type", "device_id")                  // GROUP BY multiple fields
```

## 🏭 Storage Factory Pattern

### Switching Backends

```go
// BoltDB (default)
boltStore, _ := factory.CreateStorage(storage.FactoryConfig{
    Type: storage.StorageTypeBoltDB,
    DBPath: "clipboard.db",
    Logger: logger,
    DeviceID: "device-1",
})

// SQLite (when implemented)
sqliteStore, _ := factory.CreateStorage(storage.FactoryConfig{
    Type: storage.StorageTypeSQLite,
    DBPath: "clipboard.sqlite",
    Logger: logger,
    DeviceID: "device-1",
})

// PostgreSQL (when implemented)
pgStore, _ := factory.CreateStorage(storage.FactoryConfig{
    Type: storage.StorageTypePostgres,
    ConnectionString: "postgres://user:pass@localhost/clipdb",
    Logger: logger,
    DeviceID: "device-1",
})
```

### The Beauty: **Same Query API for ALL Backends!**

```go
// This exact same query works with BoltDB, SQLite, PostgreSQL, MongoDB, etc.
results, _ := storage.NewQuery().
    Select().
    Where().
    OfType(types.TypeText).
    Contains("important").
    CreatedAfter(time.Now().AddDate(0, 0, -7)).
    OrderByCreated().
    Limit(10).
    Execute(anyStorageBackend) // Works with ANY backend!
```

## 📊 Statistics & Analytics

```go
// Comprehensive statistics
stats, err := store.GetStats()
fmt.Printf("Total items: %d\n", stats.TotalItems)
fmt.Printf("Total size: %d bytes\n", stats.TotalSize)
fmt.Printf("Average size: %.1f bytes\n", stats.AverageSize)

// Statistics by type
typeStats, _ := store.GetTypeStats()
for contentType, stat := range typeStats {
    fmt.Printf("%s: %d items, avg %.1f bytes\n", 
        contentType, stat.Count, stat.AverageSize)
}

// Top frequently used items
topItems, _ := store.GetTopOccurrences(10)
for _, item := range topItems {
    fmt.Printf("Hash %s: %d occurrences\n", 
        item.Hash, item.TotalOccurrences)
}

// Formatted summary
summary, _ := store.GetStatsSummary()
fmt.Println(summary)
```

## ⚡ Performance Best Practices

### 1. **Use ID/Hash Queries When Possible (Fastest)**
```go
// ✅ FAST: Direct ID lookup
specific, _ := storage.NewQuery().WithIDs(1, 2, 3).Execute(store)

// ✅ FAST: Direct hash lookup  
byHash, _ := storage.NewQuery().WithHashes("abc123", "def456").Execute(store)
```

### 2. **Apply Limits Early**
```go
// ✅ GOOD: Limit results to prevent memory issues
results, _ := storage.NewQuery().Limit(100).Execute(store)

// ❌ BAD: Loading all data without limits
results, _ := storage.NewQuery().Execute(store) // Could load millions of items!
```

### 3. **Use Size Filters Before Processing**
```go
// ✅ GOOD: Filter large items early
results, _ := storage.NewQuery().
    MaxSize(1024*1024). // Filter before processing
    Execute(store)
```

### 4. **Prefer Aggregation for Statistics**
```go
// ✅ FAST: Use aggregation
totalCount, _ := storage.NewQuery().Count().ExecuteAggregate(store)

// ❌ SLOW: Loading all items to count
allItems, _ := storage.NewQuery().Execute(store)
count := len(allItems)
```

### 5. **Use Pagination for Large Datasets**
```go
// ✅ GOOD: Process in chunks
pageSize := int64(50)
for page := int64(1); ; page++ {
    results, _ := storage.NewQuery().
        Paginate(page, pageSize).
        Execute(store)
    
    if len(results) == 0 {
        break // No more data
    }
    
    // Process results...
}
```

## 🔧 Advanced Features

### Transaction Support (Ready for Implementation)
```go
if txStore, ok := store.(storage.ITransactionalStorage); ok {
    tx, _ := txStore.BeginTransaction()
    
    // Multiple operations in transaction
    tx.AddContent(content1)
    tx.AddContent(content2) 
    tx.DeleteContentsByIDs([]int64{1, 2, 3})
    
    // Commit or rollback
    if err := tx.Commit(); err != nil {
        tx.Rollback()
    }
}
```

### Aggregation Support
```go
if aggStore, ok := store.(storage.IAggregateStorage); ok {
    // Native database aggregation (faster)
    results, _ := aggStore.Aggregate(storage.QueryOptions{
        Aggregate: &storage.AggregateOptions{
            Function: storage.AggCount,
            Field: "*",
            GroupBy: []string{"type"},
        },
    })
} else {
    // Falls back to in-memory aggregation
    results, _ := storage.NewQuery().
        Count().
        GroupBy("type").
        ExecuteAggregate(store)
}
```

## 🆚 Before vs. After Comparison

### ❌ **Old Way (boltdb.old.go)**
```go
// Rigid, CLI-specific options
options := config.HistoryOptions{
    Limit: 10,
    ContentType: "text", 
    Before: time.Now(),
    Reverse: true,
}
results, err := storage.GetHistory(options)
```

### ✅ **New Way (Storage-Agnostic)**
```go
// Flexible, SQL-like, storage-agnostic
results, err := storage.NewQuery().
    Select().
    Where().
    OfType(types.TypeText).
    CreatedBefore(time.Now()).
    OrderByCreatedAsc().
    Limit(10).
    Execute(anyStorageBackend) // Works with ANY backend!
```

## 🎯 What You've Achieved

### **Complete Storage Agnosticism**
- ✅ **Interface-based design**: `IStorage`, `IAggregateStorage`, `ITransactionalStorage`
- ✅ **Factory pattern**: Easy backend switching
- ✅ **Same query API**: Works with BoltDB, SQLite, PostgreSQL, MongoDB (when implemented)

### **SQL/NoSQL-like Query Language**
- ✅ **Fluent interface**: `.Select().Where().OrderBy().Limit()`
- ✅ **Complex filtering**: Time, size, content type, text search, occurrences, compression
- ✅ **Aggregation**: COUNT, SUM, AVG, MIN, MAX, GROUP BY
- ✅ **Sorting**: Multiple fields and directions
- ✅ **Pagination**: Efficient memory usage

### **Enterprise-Grade Features**
- ✅ **Performance optimized**: ID lookups, filtering, pagination
- ✅ **Statistics**: Comprehensive analytics
- ✅ **Type safety**: Compile-time validation
- ✅ **Error handling**: Proper error propagation
- ✅ **Logging**: Structured logging with zap

### **Future-Ready Architecture**
- 🚧 **Transaction support**: Interface ready
- 🚧 **Multiple backends**: Factory configured
- 🚧 **Regex search**: Framework in place
- 🚧 **Caching layer**: Easy to add
- 🚧 **Distributed storage**: Interface supports it

## 🏆 Your Next Steps

1. **Test the system**: Run the examples in `examples.go`
2. **Implement SQLite backend**: Use the factory pattern
3. **Add transaction support**: Implement the interfaces
4. **Add regex search**: Complete the text filtering
5. **Add more storage backends**: PostgreSQL, MongoDB, etc.
6. **Add caching**: Redis/Memcached layer
7. **Add sharding**: For very large datasets

You now have a **completely storage-agnostic** system that works like SQL/NoSQL but with strong typing and compile-time safety. The same query works across ANY storage backend - that's the beauty of proper abstraction!

## 📖 See Also

- `examples.go` - Comprehensive usage examples
- `boltdb.old.go` - Legacy implementation for reference
- `factory.go` - Storage backend switching
- `query.go` - SQL-like query builder implementation
