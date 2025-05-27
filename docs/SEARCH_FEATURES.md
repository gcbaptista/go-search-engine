📚 **[Documentation Index](./README.md)** | [🏠 Project Home](../README.md)

---

# Search Features

## Overview

The Go Search Engine provides advanced search capabilities with flexible field targeting, typo tolerance, filtering, and
ranking. This document covers the key search features available through the API.

## 🎯 Restrict Searchable Fields

### Overview

The `restrict_searchable_fields` feature allows you to limit search queries to specific fields, providing more targeted
search results. This is useful when you want to search only in certain fields (e.g., only in titles, or only in
descriptions).

### Usage

```bash
# Search only in title field
curl -X POST http://localhost:8080/indexes/movies/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "matrix",
    "restrict_searchable_fields": ["title"],
    "page": 1,
    "page_size": 10
  }'

# Search in multiple specific fields
curl -X POST http://localhost:8080/indexes/movies/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "action adventure",
    "restrict_searchable_fields": ["title", "description", "genres"],
    "page": 1,
    "page_size": 10
  }'
```

### Validation

- **Required**: Fields must be a subset of the index's configured `searchable_fields`
- **Error handling**: Returns error if any field is not configured as searchable
- **Optional**: When omitted, all configured searchable fields are used

### Examples

#### Valid Requests

```json
// Search using all configured searchable fields (default behavior)
{
  "query": "Matrix",
  "page": 1,
  "page_size": 10
}

// Search restricted to specific field
{
  "query": "Matrix",
  "restrict_searchable_fields": ["title"],
  "page": 1,
  "page_size": 10
}

// Search restricted to multiple fields with filters
{
  "query": "action movie",
  "restrict_searchable_fields": ["title", "description"],
  "filters": { "year_gte": 2000 },
  "page": 1,
  "page_size": 10
}
```

#### Error Response

```json
{
  "error": "restricted searchable field 'invalid_field' is not configured as a searchable field in index settings"
}
```

## 🔍 Typo Tolerance

### Overview

Advanced typo tolerance using Damerau-Levenshtein distance algorithm that handles:

- Character substitutions (e.g., "cat" → "bat")
- Character insertions (e.g., "cat" → "cart")
- Character deletions (e.g., "cart" → "cat")
- Character transpositions (e.g., "form" → "from")

**Smart Match Prevention**: Automatically prevents redundant typo matches by showing only the best quality typo match per query token per document, eliminating confusing duplicate results.

### Configuration

```json
{
  "min_word_size_for_1_typo": 4, // Words ≥4 chars allow 1 typo
  "min_word_size_for_2_typos": 7, // Words ≥7 chars allow 2 typos
  "no_typo_tolerance_fields": ["id", "category"] // Disable typos for specific fields
}
```

### Query-Time Override

You can override typo tolerance settings per search request:

```bash
curl -X POST http://localhost:8080/indexes/movies/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "matrix",
    "typo_tolerance": {
      "min_word_size_for_1_typo": 3,
      "min_word_size_for_2_typos": 6
    }
  }'
```

## 🏷️ Prefix Search

### Overview

Enables autocomplete and partial word matching. Can be configured per field.

### Configuration

```json
{
  "fields_without_prefix_search": ["id", "isbn"] // Disable prefix search for specific fields
}
```

### Usage

```bash
# Search with prefix matching (default behavior)
curl -X POST http://localhost:8080/indexes/products/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "lapt",  // Matches "laptop", "laptops", etc.
    "page": 1,
    "page_size": 10
  }'
```

## 🔧 Filtering

### Supported Filter Operators

| Operator          | Description            | Example                                   |
| ----------------- | ---------------------- | ----------------------------------------- |
| `exact`           | Exact match            | `"category_exact": "electronics"`         |
| `ne`              | Not equal              | `"status_ne": "inactive"`                 |
| `gt`              | Greater than           | `"price_gt": 100`                         |
| `gte`             | Greater than or equal  | `"year_gte": 2020`                        |
| `lt`              | Less than              | `"rating_lt": 5.0`                        |
| `lte`             | Less than or equal     | `"price_lte": 500`                        |
| `contains`        | Contains substring     | `"description_contains": "wireless"`      |
| `ncontains`       | Does not contain       | `"title_ncontains": "refurbished"`        |
| `contains_any_of` | Contains any of values | `"tags_contains_any_of": ["new", "sale"]` |

### Usage Examples

```bash
# Multiple filters
curl -X POST http://localhost:8080/indexes/products/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "laptop",
    "filters": {
      "price_gte": 500,
      "price_lte": 2000,
      "category_exact": "electronics",
      "rating_gt": 4.0,
      "tags_contains_any_of": ["gaming", "business"]
    }
  }'
```

## 📊 Ranking and Sorting

### Default Ranking

Results are ranked by:

1. **Relevance score** (exact matches, typo distance, field priority)
2. **Configured ranking criteria** (custom sort orders)
3. **Document popularity** (if available)

### Custom Ranking Criteria

```json
{
  "ranking_criteria": [
    { "field": "rating", "order": "desc" },
    { "field": "popularity", "order": "desc" },
    { "field": "year", "order": "desc" }
  ]
}
```

### Field Priority

Searchable fields are prioritized by their order in the configuration:

```json
{
  "searchable_fields": ["title", "description", "author"]
  // "title" matches score higher than "description" matches
}
```

## 🎭 Deduplication

### Overview

Remove duplicate results based on a specific field value.

### Configuration

```json
{
  "distinct_field": "title" // Deduplicate by title field
}
```

### Behavior

- Only the highest-scoring document per distinct value is returned
- Useful for removing duplicate products, articles, etc.
- Applied after filtering but before pagination

## 🔍 Search Response Format

```json
{
  "results": [
    {
      "document": {
        "documentID": "movie_1",
        "title": "The Matrix",
        "year": 1999,
        "rating": 8.7
      },
      "score": 0.95,
      "matched_fields": ["title"],
      "typo_info": {
        "has_typos": false,
        "typo_details": []
      }
    }
  ],
  "total_results": 42,
  "page": 1,
  "page_size": 10,
  "total_pages": 5,
  "search_time_ms": 15
}
```

## 💡 Best Practices

### Field Restriction

- Use `restrict_searchable_fields` for targeted searches
- Combine with filters for precise results
- Consider performance: fewer fields = faster search

### Typo Tolerance

- Adjust word size thresholds based on your data
- Disable typos for structured fields (IDs, categories)
- Use query-time overrides for special cases

### Filtering

- Use exact matches for categorical data
- Combine multiple filters for complex queries
- Consider field naming to avoid operator conflicts

### Performance

- Index only necessary fields as searchable/filterable
- Use appropriate page sizes (10-50 typically optimal)
- Monitor search times and adjust settings accordingly

## 📖 Related Documentation

- **[Search-Time vs Core Settings](./SEARCH_TIME_SETTINGS.md)** - Understanding settings that affect search behavior
- **[Typo Optimization Summary](./TYPO_OPTIMIZATION_SUMMARY.md)** - Technical details about typo tolerance performance
- **[Async API](./ASYNC_API.md)** - Managing long-running operations
- **[Analytics](./ANALYTICS.md)** - Monitoring search performance and usage
