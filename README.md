# Go Search Engine

A high-performance, full-text search engine built in Go with typo tolerance, filtering,
ranking, and prefix search capabilities.

## 🚀 Key Features

- **Full-text search** with typo tolerance (Damerau-Levenshtein distance)
- **BM25 relevance scoring** for industry-standard search quality
- **Query-time typo tolerance override** (customize minWordSizes per search request)
- **Prefix search** and autocomplete capabilities
- **Advanced filtering** with multiple operators (exact, range, contains, etc.)
- **Flexible ranking** with custom criteria and sort orders
- **Document deduplication** and Unicode support
- **Schema-agnostic** JSON document handling
- **RESTful API** with comprehensive OpenAPI 3.0 specification
- **Optimized inverted index** data structures for high performance

## 📚 Documentation

**📖 [Complete Documentation](./docs/)** - Comprehensive guides, optimization details, and development resources

| Quick Links                                               | Description                           |
| --------------------------------------------------------- | ------------------------------------- |
| [🚀 **Getting Started**](#quick-start)                    | Installation and basic usage (below)  |
| [⚙️ **Full Async API**](./docs/ASYNC_API.md)              | Complete async operations guide       |
| [⚡ **Typo Tolerance**](./docs/TYPO_TOLERANCE.md)         | Typo tolerance system                 |
| [🎯 **Analytics Guide**](./docs/ANALYTICS.md)             | Analytics and dashboard documentation |
| [🔧 **Filter Expressions**](./docs/FILTER_EXPRESSIONS.md) | Advanced boolean filtering            |
| [🔍 **Search Features**](./docs/SEARCH_FEATURES.md)       | Advanced search capabilities          |
| [🔧 **API Reference**](./api-spec.yaml)                   | Complete OpenAPI 3.0 specification    |
| [📖 **Development Guide**](./docs/CLAUDE.md)              | Coding standards and conventions      |

## Features

- **Full-text search** with typo tolerance (Damerau-Levenshtein distance)
- **Prefix search** and autocomplete capabilities
- **Advanced filtering** with multiple operators (exact, range, contains, etc.)
- **Flexible ranking** with multiple criteria and custom sort orders
- **Document deduplication** to avoid returning duplicate results
- **Unicode support** for international text
- **RESTful API** with comprehensive OpenAPI 3.0 specification
- **High performance** with optimized inverted index data structures
- **Persistent storage** with automatic data persistence
- **Schema-agnostic documents** - accept any JSON structure without field requirements
- **🚀 Full Async Operations** - All writing operations (create, update, delete) are asynchronous with real-time progress
  tracking and job management

## Architecture

The search engine follows a clean, modular architecture:

```
├── api/                    # HTTP API handlers and routing
├── cmd/search_engine/      # Main application entry point
├── config/                 # Configuration structures
├── index/                  # Inverted index implementation
├── internal/
│   ├── engine/            # Core engine orchestration
│   ├── indexing/          # Document indexing service
│   ├── search/            # Search service implementation
│   ├── tokenizer/         # Text tokenization and n-gram generation
│   ├── typoutil/          # Typo tolerance utilities
│   └── persistence/       # Data persistence layer
├── model/                 # Data models and structures
├── services/              # Service interfaces
└── store/                 # Document storage implementation
```

## Quick Start

### Prerequisites

- Go 1.23.0 or later
- Git

### Installation

```bash
git clone https://github.com/gcbaptista/go-search-engine.git
cd go-search-engine
go mod tidy
```

### Running the Server

```bash
go run cmd/search_engine/main.go
```

The server will start on port 8080 by default.

### Basic Usage

#### 1. Create an Index

```bash
curl -X POST http://localhost:8080/indexes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "movies",
    "searchable_fields": ["title", "cast", "genres"],
    "filterable_fields": ["year", "rating", "genres"],
    "ranking_criteria": [
      {"field": "popularity", "order": "desc"},
      {"field": "rating", "order": "desc"}
    ],
    "min_word_size_for_1_typo": 4,
    "min_word_size_for_2_typos": 7
  }'
```

#### 2. Add Documents

```bash
curl -X PUT http://localhost:8080/indexes/movies/documents \
  -H "Content-Type: application/json" \
  -d '[
    {
      "documentID": "movie_lotr_fellowship_2001",
      "title": "The Lord of the Rings",
      "cast": ["Elijah Wood", "Ian McKellen"],
      "genres": ["Fantasy", "Adventure"],
      "year": 2001,
      "rating": 8.8,
      "popularity": 95.5
    }
  ]'
```

**Note**: Documents are completely schema-agnostic. You can include any fields you want:

```bash
# Product document example
{
  "documentID": "product_headphones_wireless_001",
  "name": "Wireless Headphones",
  "brand": "TechBrand",
  "price": 199.99,
  "specs": {"battery": "30h", "color": "black"}
}

# Article document example
{
  "documentID": "article_breaking_news_2024_01_15",
  "headline": "Breaking News",
  "content": "Article content...",
  "author": "Jane Doe",
  "published": "2024-01-15"
}

# Custom ID formats are supported
{
  "documentID": "022ae9a1-d2ac-3238-b686-96c2a5ce26ba_en-US_MERCHANDISED_title",
  "title": "Custom Product Title",
  "locale": "en-US",
  "category": "merchandised"
}
```

The only required field is `documentID` for document identification - it can be any non-empty string.

#### 3. Search Documents

```bash
curl -X POST http://localhost:8080/indexes/movies/_search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "dark knight",
    "filters": {
      "operator": "AND",
      "filters": [
        {"field": "year", "operator": "_gte", "value": 2000},
        {"field": "rating", "operator": "_gt", "value": 8.0}
      ]
    },
    "page": 1,
    "page_size": 10
  }'
```

**Response:**

```json
{
  "hits": [
    {
      "document": {
        "documentID": "movie_dark_knight_2008",
        "title": "The Dark Knight",
        "year": 2008,
        "rating": 9.0,
        "cast": ["Christian Bale", "Heath Ledger"]
      },
      "score": 15.2,
      "field_matches": {
        "title": ["dark", "knight"]
      },
      "hit_info": {
        "num_typos": 0,
        "number_exact_words": 2
      }
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 10,
  "took": 12,
  "query_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Response Fields

- **hits**: Array of matching documents with metadata
- **total**: Total number of matches found
- **page**: Current page number (pagination)
- **page_size**: Number of results per page
- **took**: Search execution time in milliseconds
- **query_id**: Unique UUID identifying this specific search query (useful for tracking, logging, and analytics)

## API Reference

**✨ All writing operations are asynchronous** - operations return immediately with job IDs for tracking progress.

### Index Management

- `POST /indexes` - Create a new index (async, returns job ID)
- `GET /indexes` - List all indexes
- `GET /indexes/{name}` - Get index details
- `DELETE /indexes/{name}` - Delete an index (async, returns job ID)
- `PATCH /indexes/{name}/settings` - Update index settings
- `POST /indexes/{name}/rename` - Rename an index (async, returns job ID)

### Document Management

- `PUT /indexes/{name}/documents` - Add/update documents (async, returns job ID)
- `DELETE /indexes/{name}/documents` - Delete all documents from an index (async, returns job ID)
- `DELETE /indexes/{name}/documents/{id}` - Delete a specific document (async, returns job ID)

### Job Management

- `GET /jobs/{jobId}` - Get job status and progress
- `GET /indexes/{name}/jobs` - List jobs for an index
- `GET /jobs/metrics` - Get job performance metrics

### Search

- `POST /indexes/{name}/_search` - Search documents (synchronous)

### Async Operation Example

```bash
# Create index asynchronously
curl -X POST http://localhost:8080/indexes \
  -H "Content-Type: application/json" \
  -d '{"name": "products", "searchable_fields": ["title"]}'

# Response: HTTP 202 Accepted
{
  "status": "accepted",
  "message": "Index creation started for 'products'",
  "job_id": "job_12345"
}

# Poll job status
curl http://localhost:8080/jobs/job_12345

# Response: Job completed
{
  "id": "job_12345",
  "type": "create_index",
  "status": "completed",
  "index_name": "products",
  "progress": {"current": 3, "total": 3, "message": "Index creation completed"}
}
```

### Search Query Format

```json
{
  "query": "search terms",
  "filters": {
    "operator": "AND",
    "filters": [
      { "field": "field_name", "operator": "_exact", "value": "exact_value" },
      { "field": "numeric_field", "operator": "_gte", "value": 100 },
      {
        "field": "date_field",
        "operator": "_lt",
        "value": "2023-01-01T00:00:00Z"
      },
      { "field": "array_field", "operator": "_contains", "value": "value" },
      {
        "field": "array_field",
        "operator": "_contains_any_of",
        "value": ["value1", "value2"]
      }
    ]
  },
  "page": 1,
  "page_size": 10
}
```

### Filter Operators

- **Exact match**: `_exact` (default)
- **Numeric comparisons**: `_gt`, `_gte`, `_lt`, `_lte`, `_ne`
- **String operations**: `_contains`, `_ncontains`
- **Array operations**: `_contains`, `_contains_any_of`
- **Not equal**: `_ne`

## Configuration

### Index Settings

```json
{
  "name": "index_name",
  "searchable_fields": ["field1", "field2"],
  "filterable_fields": ["field1", "field3"],
  "ranking_criteria": [
    { "field": "popularity", "order": "desc" },
    { "field": "date", "order": "asc" }
  ],
  "min_word_size_for_1_typo": 4,
  "min_word_size_for_2_typos": 7,
  "fields_without_prefix_search": ["exact_field"],
  "no_typo_tolerance_fields": ["isbn", "product_code"]
}
```

#### 🔍 **Searchable Fields Priority**

**IMPORTANT**: The order of `searchable_fields` matters! The search engine uses a priority-based approach:

1. **Field 1**: Search with exact matches
2. **Field 1**: Search with typo tolerance (if enabled for this field)
3. **Field 2**: Search with exact matches
4. **Field 2**: Search with typo tolerance (if enabled for this field)
5. Continue for all remaining fields...

This ensures higher-priority fields (like `"title"`) are fully exhausted before moving to lower-priority fields (like
`"description"`).

**Example**:

```json
{
  "searchable_fields": ["title", "description", "tags"],
  "no_typo_tolerance_fields": ["tags"]
}
```

Search for "matrix":

1. Search `title` for exact "matrix" matches
2. Search `title` for typo matches ("matix", "matrx", etc.)
3. Search `description` for exact "matrix" matches
4. Search `description` for typo matches
5. Search `tags` for exact "matrix" matches only (no typos)

**Practical Use Case**:

```bash
# Create an index with priority-based search
curl -X POST http://localhost:8080/indexes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "movies",
    "searchable_fields": ["title", "plot", "cast", "genres"],
    "filterable_fields": ["year", "rating"],
    "no_typo_tolerance_fields": ["genres"],
    "fields_without_prefix_search": ["genres"],
    "ranking_criteria": [
      {"field": "rating", "order": "desc"}
    ]
  }'
```

With this configuration:

- **Title matches are highest priority** (exact + typo tolerance)
- **Plot matches are second priority** (exact + typo tolerance)
- **Cast matches are third priority** (exact + typo tolerance)
- **Genre matches are lowest priority** (exact only, no typos, no prefix search)

This ensures a search for "action" will prioritize movies with "Action" in the title over movies with "Action" in the
cast or genres.

#### ⚙️ **Field Configuration Options**

- **`fields_without_prefix_search`**: Disables n-gram/prefix search for specific fields (only whole words)
- **`no_typo_tolerance_fields`**: Disables typo tolerance for specific fields (only exact matches)
- **`distinct_field`**: Enables deduplication based on a specific field value

## Document Deduplication

The search engine supports automatic deduplication of search results based on a specified field. This is useful when you
have duplicate documents with the same content but different UUIDs.

### Configuring Deduplication

To enable deduplication, set the `distinct_field` in your index settings:

```json
{
  "name": "movies",
  "searchable_fields": ["title", "cast", "genres"],
  "filterable_fields": ["year", "rating", "genres"],
  "distinct_field": "title",
  "ranking_criteria": [{ "field": "popularity", "order": "desc" }]
}
```

### Updating Existing Index

You can enable deduplication on an existing index using the settings update endpoint:

```bash
curl -X PATCH http://localhost:8080/indexes/movies/settings \
  -H "Content-Type: application/json" \
  -d '{
    "distinct_field": "title"
  }'
```

### How It Works

- When `distinct_field` is set, the search engine will remove duplicate documents based on that field's value
- Only the **highest-scoring** document for each unique field value is kept
- Documents without the distinct field are always included (cannot be deduplicated)
- Deduplication happens after scoring and sorting, but before pagination

### Example

Without deduplication, searching for "the" might return:

- The Matrix (UUID: abc123)
- The Matrix (UUID: def456)
- The Dark Knight (UUID: ghi789)
- The Dark Knight (UUID: jkl012)

With `distinct_field: "title"`, you get:

- The Matrix (UUID: abc123) - highest scoring
- The Dark Knight (UUID: ghi789) - highest scoring

### Disabling Deduplication

To disable deduplication, set the field to an empty string:

```bash
curl -X PATCH http://localhost:8080/indexes/movies/settings \
  -H "Content-Type: application/json" \
  -d '{
    "distinct_field": ""
  }'
```

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/tokenizer
```

### Code Formatting

```bash
# Format all Go files
gofmt -w .

# Check formatting
gofmt -l .
```

### Project Structure

The project follows Go best practices:

- **Clean Architecture**: Clear separation between layers
- **Interface-Driven Design**: Dependency injection through interfaces
- **Concurrent Safety**: Proper mutex usage for shared data structures
- **Error Handling**: Comprehensive error handling throughout
- **Testing**: Unit tests for all core components

## Performance Considerations

- **Inverted Index**: Efficient O(1) term lookup
- **Concurrent Access**: Read-write mutexes for optimal performance
- **Memory Management**: Efficient data structures and minimal allocations
- **Persistence**: Optimized Gob encoding for fast serialization

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License.

## Roadmap

- [ ] Distributed search across multiple nodes
- [ ] Advanced relevance scoring algorithms
- [ ] Real-time indexing with streaming updates
- [ ] Query analytics and performance metrics
- [ ] Additional data format support (JSON, XML, CSV)
- [ ] Machine learning-based ranking improvements

## ✨ Typo Tolerance

The search engine uses **Damerau-Levenshtein distance** for typo tolerance:

- **Transposition support**: Handles adjacent character swaps
- **Better user experience**: Catches common typing mistakes
- **Performance optimized**: Fast calculation with early termination

### Common Typos Handled:

- `"teh" → "the"` (character transposition)
- `"form" → "from"` (character transposition)
- `"recieve" → "receive"` (character transposition)
- `"calendar" → "calender"` (character transposition)
