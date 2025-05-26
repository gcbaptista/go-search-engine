# Multi-Search API

The Multi-Search API allows you to execute multiple named search queries independently in a single request. Each query is executed separately and results are returned individually, making it ideal for scenarios where you need to search different fields or apply different filters simultaneously.

## Endpoint

```
POST /{indexName}/_multi_search
```

## Request Structure

```json
{
  "queries": [
    {
      "name": "query_name",
      "query": "search_terms",
      "restrict_searchable_fields": ["field1", "field2"],
      "retrivable_fields": ["field1", "field2", "field3"],
      "filters": {
        "field_gte": 100,
        "field_contains": "value"
      },
      "min_word_size_for_1_typo": 4,
      "min_word_size_for_2_typos": 7
    }
  ],
  "page": 1,
  "page_size": 10
}
```

### Request Parameters

- **queries** (required): Array of named search queries
  - **name** (required): Unique identifier for the query
  - **query** (required): Search query string
  - **restrict_searchable_fields** (optional): Subset of searchable fields to search in
  - **retrivable_fields** (optional): Subset of document fields to return
  - **filters** (optional): Query-specific filters
  - **min_word_size_for_1_typo** (optional): Override for 1-typo tolerance
  - **min_word_size_for_2_typos** (optional): Override for 2-typo tolerance
- **page** (optional): Page number for all queries (default: 1)
- **page_size** (optional): Results per page for all queries (default: 10)

## Response Structure

```json
{
  "results": {
    "query_name": {
      "hits": [...],
      "total": 10,
      "page": 1,
      "page_size": 10,
      "took": 15,
      "query_id": "uuid"
    }
  },
  "total_queries": 2,
  "processing_time_ms": 27.5
}
```

## Use Cases

### 1. Multi-Field Search

Search different terms in different fields:

```json
{
  "queries": [
    {
      "name": "title_search",
      "query": "science fiction",
      "restrict_searchable_fields": ["title"]
    },
    {
      "name": "cast_search",
      "query": "keanu reeves",
      "restrict_searchable_fields": ["cast"]
    }
  ]
}
```

### 2. Category Exploration

Search multiple categories simultaneously:

```json
{
  "queries": [
    {
      "name": "action_movies",
      "query": "action",
      "restrict_searchable_fields": ["genres"],
      "filters": {
        "year_gte": 2010,
        "rating_gte": 7.0
      }
    },
    {
      "name": "comedy_movies",
      "query": "comedy",
      "restrict_searchable_fields": ["genres"],
      "filters": {
        "year_gte": 2010,
        "rating_gte": 7.0
      }
    }
  ]
}
```

### 3. A/B Testing Search Strategies

Compare different search approaches:

```json
{
  "queries": [
    {
      "name": "exact_search",
      "query": "matrix",
      "min_word_size_for_1_typo": 0,
      "min_word_size_for_2_typos": 0
    },
    {
      "name": "fuzzy_search",
      "query": "matrix",
      "min_word_size_for_1_typo": 3,
      "min_word_size_for_2_typos": 6
    }
  ]
}
```

### 4. Field-Specific Filtering

Apply different filters to different searches:

```json
{
  "queries": [
    {
      "name": "recent_popular",
      "query": "thriller",
      "filters": {
        "year_gte": 2020,
        "rating_gte": 8.0
      },
      "retrivable_fields": ["title", "year", "rating"]
    },
    {
      "name": "classic_popular",
      "query": "thriller",
      "filters": {
        "year_lt": 2000,
        "rating_gte": 8.5
      },
      "retrivable_fields": ["title", "year", "rating", "director"]
    }
  ]
}
```

## Performance Considerations

- **Parallel Execution**: Queries are executed independently, allowing for potential parallelization
- **Individual Optimization**: Each query can be optimized separately with its own field restrictions and filters
- **Memory Usage**: Results are kept separate, avoiding the overhead of combination logic
- **Response Size**: Consider using `retrivable_fields` to limit response size when dealing with large documents

## Error Handling

The API validates:

- At least one query is required
- Query names must be unique within the request
- Query names cannot be empty
- Field restrictions must reference valid searchable fields

Common error responses:

- `400 Bad Request`: Invalid request structure or validation errors
- `404 Not Found`: Index does not exist
- `500 Internal Server Error`: Query execution errors

## Best Practices

1. **Use Descriptive Names**: Choose meaningful query names for easier result processing
2. **Limit Fields**: Use `restrict_searchable_fields` and `retrivable_fields` to improve performance
3. **Appropriate Pagination**: Set reasonable `page_size` values based on your use case
4. **Filter Early**: Apply filters to reduce the result set size
5. **Monitor Performance**: Use the `processing_time_ms` field to monitor query performance

## Example Response

```json
{
  "results": {
    "title_search": {
      "hits": [
        {
          "document": {
            "documentID": "movie_123",
            "title": "The Matrix",
            "year": 1999
          },
          "score": 0.95,
          "field_matches": {
            "title": ["matrix"]
          },
          "hit_info": {
            "num_typos": 0,
            "number_exact_words": 1
          }
        }
      ],
      "total": 1,
      "page": 1,
      "page_size": 10,
      "took": 15,
      "query_id": "uuid-123"
    },
    "cast_search": {
      "hits": [
        {
          "document": {
            "documentID": "movie_123",
            "title": "The Matrix",
            "cast": ["Keanu Reeves", "Laurence Fishburne"]
          },
          "score": 0.87,
          "field_matches": {
            "cast": ["keanu", "reeves"]
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
      "query_id": "uuid-456"
    }
  },
  "total_queries": 2,
  "processing_time_ms": 27.5
}
```
