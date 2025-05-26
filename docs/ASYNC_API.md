📚 **[Documentation Index](./README.md)** | [🏠 Project Home](../README.md)

---

# Async API Operations

## Overview

The Go Search Engine implements **full asynchronous processing for all writing operations** to eliminate timeout issues
and provide real-time progress tracking. All operations that modify data return immediately with a job ID, allowing
clients to track progress without blocking.

## 🚀 Key Features

### ✅ **Fully Async Writing Operations**

- **Index Management**: CREATE, DELETE, RENAME indexes
- **Document Operations**: ADD/UPDATE documents, DELETE documents/collections
- **Settings Updates**: ALL settings updates (both search-time and core settings)

### ✅ **Consistent API Behavior**

- All writing operations return **HTTP 202 Accepted** with job IDs
- Immediate response regardless of operation complexity
- No client timeouts on large operations
- Real-time progress tracking for all jobs

### ✅ **Comprehensive Job Management**

- Job status tracking (pending → running → completed/failed)
- Rich metadata for debugging and monitoring
- Automatic cleanup with configurable retention
- Performance monitoring and metrics

## 📋 Supported Async Operations

| Operation            | Endpoint                                | Job Type          | Description                                |
|----------------------|-----------------------------------------|-------------------|--------------------------------------------|
| Create Index         | `POST /indexes`                         | `create_index`    | Creates new search index                   |
| Delete Index         | `DELETE /indexes/{name}`                | `delete_index`    | Removes entire index                       |
| Rename Index         | `POST /indexes/{name}/rename`           | `rename_index`    | Changes index name                         |
| Add Documents        | `PUT /indexes/{name}/documents`         | `add_documents`   | Adds/updates multiple documents            |
| Delete All Documents | `DELETE /indexes/{name}/documents`      | `delete_all_docs` | Removes all documents from index           |
| Delete Document      | `DELETE /indexes/{name}/documents/{id}` | `delete_document` | Removes specific document                  |
| Update Settings      | `PATCH /indexes/{name}/settings`        | `update_settings` | Updates settings (with/without reindexing) |

## 🔄 API Response Patterns

### Async Response (HTTP 202)

All writing operations return immediately:

```json
{
  "status": "accepted",
  "message": "Document addition started for index 'products' (1000 documents)",
  "job_id": "job_abc123",
  "document_count": 1000
}
```

### Job Status Response

```json
{
  "id": "job_abc123",
  "type": "add_documents",
  "status": "running",
  "index_name": "products",
  "progress": {
    "current": 750,
    "total": 1000,
    "message": "Added 750/1000 documents"
  },
  "created_at": "2024-01-15T10:30:00Z",
  "started_at": "2024-01-15T10:30:01Z",
  "metadata": {
    "operation": "add_documents",
    "document_count": "1000"
  }
}
```

## 🛠 Job Management

### Job Status Tracking

```bash
# Get job status
GET /jobs/{jobId}

# List jobs for an index
GET /indexes/{indexName}/jobs?status=running

# Get job performance metrics
GET /jobs/metrics
```

### Job Status Values

- `pending`: Job queued but not started
- `running`: Job currently executing
- `completed`: Job finished successfully
- `failed`: Job encountered an error

## 💡 Usage Examples

### Basic Async Operation

```bash
# Start async index creation
curl -X POST http://localhost:8080/indexes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "products",
    "searchable_fields": ["title", "description"],
    "filterable_fields": ["category", "price"]
  }'

# Response: HTTP 202
{
  "status": "accepted",
  "message": "Index creation started for 'products'",
  "job_id": "job_12345"
}

# Poll job status
curl http://localhost:8080/jobs/job_12345
```

### Async Document Operations

```bash
# Add documents asynchronously
curl -X PUT http://localhost:8080/indexes/products/documents \
  -H "Content-Type: application/json" \
  -d '[
    {"documentID": "prod_1", "title": "Laptop", "price": 999},
    {"documentID": "prod_2", "title": "Mouse", "price": 29}
  ]'

# Response: HTTP 202
{
  "status": "accepted",
  "message": "Document addition started for index 'products' (2 documents)",
  "job_id": "job_67890",
  "document_count": 2
}
```

### Settings Updates

Settings updates are automatically handled based on the type of change:

```bash
# Search-time settings (fast, no reindexing)
curl -X PATCH http://localhost:8080/indexes/products/settings \
  -H "Content-Type: application/json" \
  -d '{
    "min_word_size_for_1_typo": 3,
    "no_typo_tolerance_fields": ["category"]
  }'

# Core settings (slow, requires reindexing)
curl -X PATCH http://localhost:8080/indexes/products/settings \
  -H "Content-Type: application/json" \
  -d '{
    "searchable_fields": ["title", "description", "brand"],
    "filterable_fields": ["category", "price", "rating"]
  }'
```

Both return HTTP 202 with job IDs, but core settings will take longer to complete.

## 🎯 Benefits

### **No Client Timeouts**

- API returns immediately with job ID
- Client can poll for status updates
- Long-running operations don't block HTTP connections

### **Progress Visibility**

- Real-time progress reporting
- Batch processing with progress updates
- Clear status messages and error reporting

### **Better Resource Management**

- Concurrent job limiting prevents resource exhaustion
- Memory-efficient batch processing
- Automatic cleanup of completed jobs

### **Consistent Experience**

- All writing operations behave the same way
- Predictable API responses
- No special handling needed for different operation types

## 🔍 Monitoring and Debugging

### Job Metrics

```bash
# Get overall job performance metrics
curl http://localhost:8080/jobs/metrics
```

Returns information about:

- Success rates across all operations
- Average execution times
- Current workload monitoring
- Detailed job statistics

### Error Handling

Failed jobs include detailed error information:

```json
{
  "id": "job_failed123",
  "status": "failed",
  "error": "Failed to add documents: invalid document format",
  "completed_at": "2024-01-15T10:35:00Z"
}
```

## 📖 Related Documentation

- **[Search-Time vs Core Settings](./SEARCH_TIME_SETTINGS.md)** - Understanding different types of settings updates
- **[Analytics](./ANALYTICS.md)** - Performance monitoring and analytics
- **[Dashboard Guide](./DASHBOARD_GUIDE.md)** - Using the web dashboard with async operations
