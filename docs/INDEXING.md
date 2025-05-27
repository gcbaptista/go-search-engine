# Document Indexing System

📚 **[Documentation Index](./README.md)** | [🏠 Project Home](../README.md)

---

## Overview

The Go Search Engine uses an inverted index to enable fast full-text search across document collections. The indexing system processes documents by extracting text content, tokenizing it, and building searchable data structures that power the search functionality.

### What is Indexing?

Indexing is the process of analyzing documents and creating data structures that enable fast search operations. When you add documents to the search engine, the indexing system:

1. **Extracts text content** from searchable fields
2. **Tokenizes text** into individual terms and n-grams
3. **Builds inverted index** mapping terms to documents
4. **Stores documents** for retrieval and filtering

### Key Features

- **Schema-agnostic**: Index any JSON document structure
- **Field-specific configuration**: Control which fields are searchable vs filterable
- **Automatic optimization**: Efficient processing for both small and large batches
- **Real-time updates**: Add, update, or delete documents instantly
- **Concurrent operations**: Thread-safe indexing and searching

## How Indexing Works

### Document Structure

Documents are JSON objects with a required `documentID` field:

```go
document := model.Document{
    "documentID": "product_123",        // Required: unique identifier
    "title":      "Wireless Headphones", // Searchable field
    "description": "High-quality audio...", // Searchable field
    "category":   "Electronics",        // Filterable field
    "price":      99.99,               // Filterable field
    "tags":       []string{"audio", "wireless"}, // Array field
}
```

### Indexing Process

When a document is indexed:

#### 1. Field Processing

The system processes each field based on your index configuration:

```go
settings := config.IndexSettings{
    SearchableFields: []string{"title", "description", "tags"},
    FilterableFields: []string{"category", "price"},
}
```

- **Searchable fields**: Text is tokenized and added to the inverted index
- **Filterable fields**: Values are indexed for exact-match filtering
- **Other fields**: Stored but not indexed (retrievable in search results)

#### 2. Tokenization

Text content is broken down into searchable tokens:

```go
// Input text: "Wireless Headphones"
// Regular tokens: ["wireless", "headphones"]
// N-gram tokens: ["w", "wi", "wir", "wire", "wirel", "wirele", ...]
```

The system supports two tokenization modes:

- **Regular tokenization**: Whole words for exact matching
- **N-gram tokenization**: Prefix matching for autocomplete

#### 3. Inverted Index Construction

Tokens are mapped to documents in the inverted index:

```
Token "wireless" → [
    {DocID: 123, Field: "title", Score: 1.0},
    {DocID: 456, Field: "description", Score: 2.0},
]
```

Each entry includes:

- **Document ID**: Internal reference to the document
- **Field name**: Which field contained the token
- **Score**: Term frequency or relevance score

## Adding Documents

### Basic Usage

```go
// Single document
doc := model.Document{
    "documentID": "doc1",
    "title": "Example Document",
    "content": "This is the document content",
}

err := indexInstance.AddDocuments([]model.Document{doc})
if err != nil {
    log.Printf("Failed to index document: %v", err)
}
```

### Batch Operations

The system automatically optimizes based on batch size:

```go
// Small batches (< 100 docs): Optimized for low latency
docs := generateDocuments(50)
err := indexInstance.AddDocuments(docs)

// Large batches (≥ 100 docs): Optimized for high throughput
largeDocs := generateDocuments(5000)
err := indexInstance.AddDocuments(largeDocs) // Uses bulk processing
```

### Document Updates

Updating a document with the same `documentID` replaces the previous version:

```go
// Original document
original := model.Document{
    "documentID": "product_123",
    "title": "Old Title",
    "price": 50.0,
}

// Updated document
updated := model.Document{
    "documentID": "product_123", // Same ID
    "title": "New Title",        // Updated field
    "price": 75.0,              // Updated field
    "category": "Electronics",   // New field
}

err := indexInstance.AddDocuments([]model.Document{updated})
// The old document is automatically removed from the index
```

## Document Deletion

### Delete Single Document

```go
err := indexInstance.DeleteDocument("product_123")
if err != nil {
    log.Printf("Failed to delete document: %v", err)
}
```

### Delete All Documents

```go
err := indexInstance.DeleteAllDocuments()
if err != nil {
    log.Printf("Failed to clear index: %v", err)
}
```

## Field Configuration

### Searchable Fields

Fields that support full-text search with tokenization:

```go
settings := config.IndexSettings{
    SearchableFields: []string{"title", "description", "content"},
}
```

**Use cases**:

- Product titles and descriptions
- Article content
- User-generated content
- Any text that needs fuzzy matching

### Filterable Fields

Fields that support exact-match filtering:

```go
settings := config.IndexSettings{
    FilterableFields: []string{"category", "brand", "price", "status"},
}
```

**Use cases**:

- Categories and tags
- Numerical values (prices, ratings)
- Dates and timestamps
- Status fields (active, inactive)

### Prefix Search Configuration

Control which fields support prefix/autocomplete search:

```go
settings := config.IndexSettings{
    SearchableFields: []string{"title", "description", "tags"},
    FieldsWithoutPrefixSearch: []string{"description"}, // Exact words only
}
```

- Fields not in `FieldsWithoutPrefixSearch`: Support prefix matching
- Fields in `FieldsWithoutPrefixSearch`: Whole words only (better performance)

## Advanced Configuration

### Custom Bulk Indexing

For large-scale operations, you can customize the indexing process:

```go
config := indexing.BulkIndexingConfig{
    BatchSize:       1000,             // Documents per batch
    WorkerCount:     runtime.NumCPU(), // Parallel workers
    FlushInterval:   5 * time.Second,  // How often to flush
    MemoryThreshold: 500,              // MB before forced flush
    ProgressCallback: func(processed, total int, message string) {
        log.Printf("Indexed %d/%d documents", processed, total)
    },
}

bulkIndexer := indexing.NewBulkIndexer(service, config)
err := bulkIndexer.BulkAddDocuments(documents)
```

### Performance Tuning

The system automatically adapts to your workload:

- **Small batches**: Low-latency processing for real-time updates
- **Large batches**: High-throughput processing with parallel workers
- **Memory management**: Automatic flushing to prevent memory issues

## Reindexing

### When Reindexing is Needed

Reindexing rebuilds the entire index and is required when:

- **Searchable fields change**: Adding or removing searchable fields
- **Tokenization settings change**: Modifying prefix search configuration
- **Index corruption**: Recovering from data inconsistencies

### Automatic Reindexing

The system automatically triggers reindexing when needed:

```go
// This may trigger reindexing if searchable fields changed
jobID, err := engine.UpdateIndexSettingsWithAsyncReindex(indexName, newSettings)
if err != nil {
    log.Printf("Failed to update settings: %v", err)
    return
}

// Monitor progress
job, err := engine.GetJob(jobID)
if err == nil && job.Progress != nil {
    log.Printf("Reindex progress: %d/%d", job.Progress.Processed, job.Progress.Total)
}
```

### Manual Reindexing

Force a complete reindex:

```go
config := indexing.DefaultBulkIndexingConfig()
config.ProgressCallback = func(processed, total int, message string) {
    log.Printf("Reindexing: %d/%d - %s", processed, total, message)
}

err := indexInstance.BulkReindex(config)
if err != nil {
    log.Printf("Reindex failed: %v", err)
}
```

## Data Types and Processing

### Supported Field Types

```go
document := model.Document{
    "documentID": "example",

    // String fields
    "title": "Product Title",

    // String arrays
    "tags": []string{"electronics", "gadgets"},

    // Mixed arrays (converted to strings)
    "categories": []interface{}{"tech", "mobile"},

    // Numbers (for filtering)
    "price": 99.99,
    "rating": 4.5,

    // Booleans (for filtering)
    "inStock": true,

    // Nested objects (flattened)
    "metadata": map[string]interface{}{
        "brand": "TechCorp",
        "model": "X1",
    },
}
```

### Text Processing

The indexing system handles text processing automatically:

1. **Normalization**: Convert to lowercase, handle Unicode
2. **Tokenization**: Split into words and n-grams
3. **Deduplication**: Remove duplicate tokens within the same field
4. **Frequency calculation**: Count term occurrences for scoring

## Integration Examples

### REST API Handler

```go
func (h *Handler) AddDocuments(c *gin.Context) {
    var docs []model.Document
    if err := c.ShouldBindJSON(&docs); err != nil {
        c.JSON(400, gin.H{"error": "Invalid JSON"})
        return
    }

    // Validate documents
    for i, doc := range docs {
        if _, exists := doc["documentID"]; !exists {
            c.JSON(400, gin.H{"error": fmt.Sprintf("Document %d missing documentID", i)})
            return
        }
    }

    // Index documents
    indexName := c.Param("indexName")
    err := h.engine.GetIndex(indexName).AddDocuments(docs)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{
        "message": fmt.Sprintf("Successfully indexed %d documents", len(docs)),
        "indexed": len(docs),
    })
}
```

### Async Bulk Import

```go
func BulkImportFromFile(filename string, indexInstance *engine.IndexInstance) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    var documents []model.Document
    decoder := json.NewDecoder(file)

    for decoder.More() {
        var doc model.Document
        if err := decoder.Decode(&doc); err != nil {
            return err
        }
        documents = append(documents, doc)

        // Process in batches of 1000
        if len(documents) >= 1000 {
            if err := indexInstance.AddDocuments(documents); err != nil {
                return err
            }
            documents = documents[:0] // Reset slice
        }
    }

    // Process remaining documents
    if len(documents) > 0 {
        return indexInstance.AddDocuments(documents)
    }

    return nil
}
```

## Monitoring and Observability

### Progress Tracking

Monitor indexing operations with progress callbacks:

```go
config := indexing.DefaultBulkIndexingConfig()
config.ProgressCallback = func(processed, total int, message string) {
    percentage := float64(processed) / float64(total) * 100
    log.Printf("Indexing progress: %.1f%% (%d/%d) - %s",
        percentage, processed, total, message)
}
```

### Performance Metrics

Key metrics to monitor:

- **Indexing throughput**: Documents processed per second
- **Memory usage**: Current and peak memory consumption
- **Index size**: Number of documents and unique tokens
- **Error rates**: Failed indexing operations

### Logging

The system provides detailed logging:

```
2025/05/27 16:43:51 Starting bulk indexing of 5000 documents with 10 workers
2025/05/27 16:43:51 Processing batch 1-1000 (1000 documents)
2025/05/27 16:43:51 Processing batch 1001-2000 (1000 documents)
2025/05/27 16:43:51 Flushing 5060 token updates and 5000 document updates
2025/05/27 16:43:51 Bulk indexing completed: 5000 documents in 81.26ms (61534.14 docs/sec)
```

## Best Practices

### Document Design

1. **Use meaningful documentIDs**: Unique, stable identifiers
2. **Structure fields appropriately**: Separate searchable from filterable content
3. **Normalize text content**: Consistent formatting and encoding
4. **Avoid deeply nested objects**: Flatten complex structures

### Performance Optimization

1. **Batch operations**: Group multiple documents for better throughput
2. **Configure fields wisely**: Only make necessary fields searchable
3. **Monitor memory usage**: Adjust batch sizes for your environment
4. **Use async operations**: For large imports, use background jobs

### Error Handling

1. **Validate documents**: Check for required fields before indexing
2. **Handle partial failures**: Continue processing when some documents fail
3. **Implement retry logic**: For transient failures
4. **Monitor index health**: Regular consistency checks

## Troubleshooting

### Common Issues

**Document not found in search results**:

- Verify the field is configured as searchable
- Check if the document was successfully indexed
- Ensure tokenization is working correctly

**Slow indexing performance**:

- Increase batch size for large operations
- Reduce the number of searchable fields
- Check memory usage and adjust thresholds

**Memory issues**:

- Reduce batch size and memory thresholds
- Process documents in smaller chunks
- Monitor memory usage during operations

**Index inconsistencies**:

- Perform a full reindex
- Check for concurrent modification issues
- Verify document ID uniqueness

---

## Related Documentation

- [**Search Features**](./SEARCH_FEATURES.md) - How to search indexed documents
- [**Search-Time Settings**](./SEARCH_TIME_SETTINGS.md) - Configuration that affects search behavior
- [**Async API Operations**](./ASYNC_API.md) - Background job management
- [**API Specification**](../api-spec.yaml) - REST API endpoints

---

_📌 **Note**: The indexing system is designed to be simple and efficient out of the box. Most applications can use the default configuration without any tuning._
