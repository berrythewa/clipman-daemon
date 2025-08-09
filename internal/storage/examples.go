package storage

import (
	"fmt"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// This file contains examples of how to use the new storage-agnostic query system
// These are meant to be documentation and usage examples

// ExampleBasicQueries demonstrates basic query operations
func ExampleBasicQueries() {
	logger := zap.NewNop()
	
	// Create storage using factory
	factory := NewStorageFactory(logger)
	config := DefaultStorageConfig(StorageTypeBoltDB, "example.db", logger, "device-123")
	
	storage, err := factory.CreateStorage(config)
	if err != nil {
		panic(err)
	}
	defer storage.Close()

	// Example 1: Get latest 10 items
	fmt.Println("=== Example 1: Latest 10 items ===")
	latest, _ := NewQuery().
		Select().
		OrderByCreated().
		Limit(10).
		Execute(storage)
	
	fmt.Printf("Found %d latest items\n", len(latest))

	// Example 2: Search for text content containing "hello"
	fmt.Println("\n=== Example 2: Text search ===")
	textResults, _ := NewQuery().
		Select().
		Where().
		OfType(types.TypeText).
		Contains("hello").
		OrderByCreated().
		Execute(storage)
	
	fmt.Printf("Found %d items containing 'hello'\n", len(textResults))

	// Example 3: Get images larger than 1MB from last week
	fmt.Println("\n=== Example 3: Large images from last week ===")
	weekAgo := time.Now().AddDate(0, 0, -7)
	imageResults, _ := NewQuery().
		Select().
		Where().
		OfType(types.TypeImage).
		MinSize(1024 * 1024). // 1MB
		CreatedAfter(weekAgo).
		OrderBySize().
		Execute(storage)
	
	fmt.Printf("Found %d large images from last week\n", len(imageResults))

	// Example 4: Paginated results
	fmt.Println("\n=== Example 4: Paginated results ===")
	page1, _ := NewQuery().
		Select().
		OrderByCreated().
		Paginate(1, 5). // Page 1, 5 items per page
		Execute(storage)
	
	page2, _ := NewQuery().
		Select().
		OrderByCreated().
		Paginate(2, 5). // Page 2, 5 items per page
		Execute(storage)
	
	fmt.Printf("Page 1: %d items, Page 2: %d items\n", len(page1), len(page2))

	// Example 5: Complex filtering
	fmt.Println("\n=== Example 5: Complex filtering ===")
	complexResults, _ := NewQuery().
		Select().
		Where().
		OfTypes(types.TypeText, types.TypeHTML). // Multiple types
		SizeBetween(100, 10000).                 // Size range
		WithOccurrences().                       // Has multiple occurrences
		CreatedBetween(
			time.Now().AddDate(0, 0, -30), // 30 days ago
			time.Now().AddDate(0, 0, -1),  // Yesterday
		).
		OrderByOccurrences().
		Limit(20).
		Execute(storage)
	
	fmt.Printf("Found %d complex filtered items\n", len(complexResults))
}

// ExampleAggregationQueries demonstrates aggregation operations
func ExampleAggregationQueries() {
	logger := zap.NewNop()
	
	// Create storage using factory
	factory := NewStorageFactory(logger)
	config := DefaultStorageConfig(StorageTypeBoltDB, "example.db", logger, "device-123")
	
	storage, err := factory.CreateStorage(config)
	if err != nil {
		panic(err)
	}
	defer storage.Close()

	// Example 1: Count all items
	fmt.Println("=== Example 1: Count all items ===")
	countResults, _ := NewQuery().
		Select().
		Count().
		ExecuteAggregate(storage)
	
	if len(countResults) > 0 {
		fmt.Printf("Total items: %v\n", countResults[0].Value)
	}

	// Example 2: Count items by type
	fmt.Println("\n=== Example 2: Count by content type ===")
	countByType, _ := NewQuery().
		Select().
		Count().
		GroupBy("type").
		ExecuteAggregate(storage)
	
	for _, result := range countByType {
		fmt.Printf("Type %v: %v items\n", 
			result.GroupBy["type"], result.Value)
	}

	// Example 3: Average size by type
	fmt.Println("\n=== Example 3: Average size by type ===")
	avgSizeByType, _ := NewQuery().
		Select().
		Avg("size").
		GroupBy("type").
		ExecuteAggregate(storage)
	
	for _, result := range avgSizeByType {
		fmt.Printf("Type %v: avg size %.1f bytes\n", 
			result.GroupBy["type"], result.Value)
	}

	// Example 4: Total storage usage
	fmt.Println("\n=== Example 4: Total storage usage ===")
	totalSize, _ := NewQuery().
		Select().
		Sum("size").
		ExecuteAggregate(storage)
	
	if len(totalSize) > 0 {
		fmt.Printf("Total storage: %v bytes\n", totalSize[0].Value)
	}

	// Example 5: Most recent and oldest items
	fmt.Println("\n=== Example 5: Most recent and oldest items ===")
	newest, _ := NewQuery().
		Select().
		Last().
		ExecuteAggregate(storage)
	
	oldest, _ := NewQuery().
		Select().
		First().
		ExecuteAggregate(storage)
	
	if len(newest) > 0 && len(oldest) > 0 {
		newestContent := newest[0].Value.(*types.ClipboardContent)
		oldestContent := oldest[0].Value.(*types.ClipboardContent)
		
		fmt.Printf("Newest: ID %d, created %v\n", 
			newestContent.Id, newestContent.Created)
		fmt.Printf("Oldest: ID %d, created %v\n", 
			oldestContent.Id, oldestContent.Created)
	}
}

// ExampleDifferentStorageBackends shows how to use different storage backends
func ExampleDifferentStorageBackends() {
	logger := zap.NewNop()
	factory := NewStorageFactory(logger)

	// BoltDB storage
	fmt.Println("=== BoltDB Storage ===")
	boltConfig := DefaultStorageConfig(StorageTypeBoltDB, "bolt.db", logger, "device-1")
	boltStorage, err := factory.CreateStorage(boltConfig)
	if err != nil {
		fmt.Printf("Error creating BoltDB storage: %v\n", err)
	} else {
		defer boltStorage.Close()
		fmt.Println("BoltDB storage created successfully")
		
		// Use the same query interface regardless of backend
		results, _ := NewQuery().
			Select().
			OrderByCreated().
			Limit(5).
			Execute(boltStorage)
		
		fmt.Printf("Found %d items in BoltDB\n", len(results))
	}

	// SQLite storage (when implemented)
	fmt.Println("\n=== SQLite Storage (Future) ===")
	sqliteConfig := DefaultStorageConfig(StorageTypeSQLite, "sqlite.db", logger, "device-1")
	_, err = factory.CreateStorage(sqliteConfig)
	if err != nil {
		fmt.Printf("SQLite not yet implemented: %v\n", err)
	}

	// The beauty of the storage-agnostic design:
	// The same query builder works with ANY storage backend!
	fmt.Println("\n=== Storage Agnostic Queries ===")
	fmt.Println("// This query works with ANY storage backend:")
	fmt.Println("results, _ := NewQuery().")
	fmt.Println("    Select().")
	fmt.Println("    Where().")
	fmt.Println("    OfType(types.TypeText).")
	fmt.Println("    Contains(\"important\").")
	fmt.Println("    OrderByCreated().")
	fmt.Println("    Limit(10).")
	fmt.Println("    Execute(anyStorageBackend)")
}

// ExampleTransactionSupport shows how transaction support would work
func ExampleTransactionSupport() {
	logger := zap.NewNop()
	factory := NewStorageFactory(logger)
	config := DefaultStorageConfig(StorageTypeBoltDB, "example.db", logger, "device-123")
	
	storage, err := factory.CreateStorage(config)
	if err != nil {
		panic(err)
	}
	defer storage.Close()

	// Check if storage supports transactions
	if txStorage, ok := storage.(ITransactionalStorage); ok {
		fmt.Println("=== Transaction Support ===")
		
		// Begin transaction (hypothetical - not implemented yet)
		tx, err := txStorage.BeginTransaction()
		if err != nil {
			fmt.Printf("Error beginning transaction: %v\n", err)
			return
		}

		// Perform multiple operations in transaction
		content1 := &types.ClipboardContent{
			Type: types.TypeText,
			Data: []byte("Transaction test 1"),
		}
		content2 := &types.ClipboardContent{
			Type: types.TypeText,
			Data: []byte("Transaction test 2"),
		}

		// Add content in transaction
		tx.AddContent(content1)
		tx.AddContent(content2)
		
		// Query within transaction
		results, _ := tx.Query(QueryOptions{Limit: 10})
		fmt.Printf("Items in transaction: %d\n", len(results))

		// Commit or rollback
		if err := tx.Commit(); err != nil {
			fmt.Printf("Transaction commit failed: %v\n", err)
			tx.Rollback()
		} else {
			fmt.Println("Transaction committed successfully")
		}
	} else {
		fmt.Println("Storage does not support transactions")
	}
}

// ExampleErrorHandling shows proper error handling patterns
func ExampleErrorHandling() {
	logger := zap.NewNop()
	factory := NewStorageFactory(logger)

	fmt.Println("=== Error Handling Examples ===")

	// 1. Invalid storage type
	invalidConfig := FactoryConfig{
		Type:     "invalid-type",
		DBPath:   "test.db",
		Logger:   logger,
		DeviceID: "device-1",
	}
	
	_, err := factory.CreateStorage(invalidConfig)
	if err != nil {
		fmt.Printf("Expected error for invalid storage type: %v\n", err)
	}

	// 2. Query validation error
	_, err = NewQuery().
		GroupBy("type"). // GroupBy without aggregate function
		Build()
	
	if err != nil {
		fmt.Printf("Query validation error: %v\n", err)
	}

	// 3. Missing required configuration
	missingConfig := FactoryConfig{
		Type:     StorageTypeBoltDB,
		// Missing DBPath
		Logger:   logger,
		DeviceID: "device-1",
	}
	
	_, err = factory.CreateStorage(missingConfig)
	if err != nil {
		fmt.Printf("Expected error for missing DB path: %v\n", err)
	}

	fmt.Println("Error handling examples completed")
}

// ExamplePerformanceOptimizations shows performance-aware usage patterns
func ExamplePerformanceOptimizations() {
	logger := zap.NewNop()
	factory := NewStorageFactory(logger)
	config := DefaultStorageConfig(StorageTypeBoltDB, "example.db", logger, "device-123")
	
	storage, err := factory.CreateStorage(config)
	if err != nil {
		panic(err)
	}
	defer storage.Close()

	fmt.Println("=== Performance Optimization Examples ===")

	// 1. Use specific ID/Hash queries when possible (most efficient)
	fmt.Println("1. Efficient ID-based queries:")
	ids := []int64{1, 2, 3, 4, 5}
	specificResults, _ := NewQuery().
		WithIDs(ids...).
		Execute(storage)
	fmt.Printf("   Found %d items by ID (very fast)\n", len(specificResults))

	// 2. Limit results early to reduce memory usage
	fmt.Println("2. Limited queries to reduce memory:")
	limitedResults, _ := NewQuery().
		Select().
		Limit(100). // Always use reasonable limits
		Execute(storage)
	fmt.Printf("   Limited to %d items (memory efficient)\n", len(limitedResults))

	// 3. Use size filters before content processing
	fmt.Println("3. Size filtering before processing:")
	sizeFilteredResults, _ := NewQuery().
		Select().
		Where().
		MaxSize(1024 * 1024). // Filter large items early
		Execute(storage)
	fmt.Printf("   Size-filtered to %d items (faster processing)\n", len(sizeFilteredResults))

	// 4. Use pagination for large result sets
	fmt.Println("4. Pagination for large datasets:")
	pageSize := int64(50)
	totalProcessed := 0
	
	for page := int64(1); page <= 5; page++ { // Process first 5 pages
		pageResults, _ := NewQuery().
			Select().
			OrderByCreated().
			Paginate(page, pageSize).
			Execute(storage)
		
		if len(pageResults) == 0 {
			break // No more results
		}
		
		totalProcessed += len(pageResults)
		// Process page results...
	}
	fmt.Printf("   Processed %d items across multiple pages (memory efficient)\n", totalProcessed)

	// 5. Prefer aggregation for statistics
	fmt.Println("5. Use aggregation for statistics:")
	stats, _ := NewQuery().
		Count().
		ExecuteAggregate(storage)
	
	if len(stats) > 0 {
		fmt.Printf("   Total count via aggregation: %v (faster than loading all)\n", stats[0].Value)
	}

	fmt.Println("Performance optimization examples completed")
}
