# Filter Expressions with AND/OR Logic

Filter expressions provide advanced filtering capabilities with AND/OR logic and explicit scoring, inspired by Algolia's
filter syntax. This extends the basic filter scoring to support complex boolean logic.

## Overview

Filter expressions allow you to:

- Combine multiple filter conditions with AND/OR logic
- Nest filter groups for complex boolean expressions
- Assign explicit scores to individual filter conditions
- Create complex boolean expressions with AND/OR logic

## Basic Structure

```json
{
  "filters": {
    "operator": "AND|OR",
    "filters": [
      {
        "field": "genre",
        "operator": "_exact",
        "value": "Action",
        "score": 2.0
      }
    ],
    "groups": [
      {
        "operator": "OR",
        "filters": [...]
      }
    ]
  }
}
```

## Simple Examples

### OR Logic

Match documents with either Action OR Comedy genre:

```json
{
  "query": "movie",
  "filters": {
    "operator": "OR",
    "filters": [
      { "field": "genre", "value": "Action", "score": 2.0 },
      { "field": "genre", "value": "Comedy", "score": 1.5 }
    ]
  }
}
```

### AND Logic

Match documents with Action genre AND premium status:

```json
{
  "query": "movie",
  "filters": {
    "operator": "AND",
    "filters": [
      { "field": "genre", "value": "Action", "score": 2.0 },
      { "field": "is_premium", "value": true, "score": 3.0 }
    ]
  }
}
```

## Complex Nested Expressions

### Nested Groups

Match documents with (Action OR Comedy) AND (premium OR high rating):

```json
{
  "query": "movie",
  "filters": {
    "operator": "AND",
    "groups": [
      {
        "operator": "OR",
        "filters": [
          { "field": "genre", "value": "Action", "score": 2.0 },
          { "field": "genre", "value": "Comedy", "score": 1.5 }
        ]
      },
      {
        "operator": "OR",
        "filters": [
          { "field": "is_premium", "value": true, "score": 3.0 },
          { "field": "rating", "operator": "_gte", "value": 8.0, "score": 2.5 }
        ]
      }
    ]
  }
}
```

## Algolia-Inspired Complex Expression

Based on the Algolia query pattern, here's a complex real-world example:

```json
{
  "query": "thriller",
  "filters": {
    "operator": "AND",
    "groups": [
      {
        "operator": "OR",
        "filters": [
          { "field": "suggestionType", "value": "programme", "score": 0.5 },
          { "field": "suggestionType", "value": "series", "score": 0.5 }
        ]
      },
      {
        "operator": "OR",
        "filters": [
          {
            "field": "filters",
            "operator": "_contains",
            "value": "plat_pc_dev_computer",
            "score": 1.0
          },
          {
            "field": "filters",
            "operator": "_contains",
            "value": "plat_dev_all",
            "score": 1.0
          }
        ]
      },
      {
        "operator": "OR",
        "filters": [
          {
            "field": "filters",
            "operator": "_contains",
            "value": "prop_nbcuott",
            "score": 0.8
          },
          {
            "field": "filters",
            "operator": "_contains",
            "value": "prop_all",
            "score": 0.8
          }
        ]
      },
      {
        "operator": "OR",
        "filters": [
          {
            "field": "filters",
            "operator": "_contains",
            "value": "content_format_longform",
            "score": 0.6
          },
          {
            "field": "filters",
            "operator": "_contains",
            "value": "content_format_trailer",
            "score": 0.6
          }
        ]
      }
    ]
  }
}
```

## Filter Operator Paradigm

The Go Search Engine uses **explicit operators** in filter expressions:

### Filter Expression Approach

```json
{
  "filters": {
    "operator": "AND",
    "filters": [
      {
        "field": "genre",
        "operator": "_contains",
        "value": "Action",
        "score": 2.0
      },
      {
        "field": "rating",
        "operator": "_gte",
        "value": 8.0,
        "score": 1.5
      }
    ]
  }
}
```

### Operator Auto-Detection

When no `operator` is specified in a `FilterCondition`, the system auto-detects based on the document field type:

- **Arrays** (`[]string`, `[]interface{}`): Defaults to `_contains`
- **Simple values** (string, number, boolean): Defaults to `_exact`

### Field Name Validation

The system still validates field names to prevent conflicts with operator suffixes. This ensures that field names like
`"price_gte"` don't cause confusion with the `_gte` operator.

## Ranking with Filter Scores

### Using `~filters` Ranking Criterion

When you want to sort results primarily by filter match scores, use the special `~filters` field in ranking criteria:

```json
{
  "ranking_criteria": [
    {
      "field": "~filters",
      "order": "desc"
    },
    {
      "field": "~score",
      "order": "desc"
    }
  ]
}
```

**Important**: When using `~filters` as the primary ranking criterion, ensure your filter expressions include meaningful
scores:

```json
{
  "filters": {
    "operator": "OR",
    "filters": [
      {
        "field": "is_premium",
        "value": true,
        "score": 10.0 // High score for premium content
      },
      {
        "field": "rating",
        "operator": "_gte",
        "value": 8.0,
        "score": 5.0 // Medium score for high ratings
      },
      {
        "field": "genre",
        "value": "Action",
        "score": 2.0 // Low score for genre match
      }
    ]
  }
}
```

This approach allows you to:

1. **Boost premium content** with high filter scores
2. **Prioritize quality** with rating-based scores
3. **Fine-tune relevance** with genre preferences
4. **Combine with search relevance** using `~score` as secondary criterion

### Filter-Only Searches

For searches that rely entirely on filter scoring (e.g., recommendation systems), you can:

1. Use an empty query string: `"query": ""`
2. Set ranking criteria to prioritize filters: `[{"field": "~filters", "order": "desc"}]`
3. Design filter expressions with meaningful score distributions

## Filter Operators

All standard filter operators are supported in expressions:

- `_exact` (default): Exact match
- `_contains`: String contains or array contains
- `_gte`, `_gt`: Greater than (equal)
- `_lte`, `_lt`: Less than (equal)
- `_ne`: Not equal
- `_ncontains`: Does not contain
- `_contains_any_of`: Array contains any of the values

## Auto-Detection

If no operator is specified, the system auto-detects based on the document field type:

- Array fields ([]string, []interface{}): Uses `_contains`
- Other fields: Uses `_exact`

## Scoring Logic

### Individual Conditions

Each filter condition can have an explicit `score` value. When the condition matches, this score is added to the
document's total filter score.

### Group Scoring

- **AND groups**: All conditions must match, scores from all matching conditions are summed
- **OR groups**: At least one condition must match, scores from all matching conditions are summed

### Total Filter Score

The final filter score is the sum of all matching filter expression scores.

## Multi-Search Support

Filter expressions work seamlessly with multi-search:

```json
{
  "queries": [
    {
      "name": "premium_content",
      "query": "action",
      "filters": {
        "operator": "AND",
        "filters": [
          { "field": "genre", "value": "Action", "score": 2.0 },
          { "field": "is_premium", "value": true, "score": 5.0 }
        ]
      }
    },
    {
      "name": "platform_specific",
      "query": "comedy",
      "filters": {
        "operator": "OR",
        "filters": [
          {
            "field": "filters",
            "operator": "_contains",
            "value": "plat_pc_dev_computer",
            "score": 1.0
          },
          {
            "field": "filters",
            "operator": "_contains",
            "value": "plat_mobile_dev_phone",
            "score": 0.8
          }
        ]
      }
    }
  ]
}
```

## Performance Considerations

1. **Nested Depth**: Keep nesting levels reasonable (max 3-4 levels)
2. **Condition Count**: Large numbers of OR conditions may impact performance
3. **Field Indexing**: Ensure filtered fields are marked as filterable in index settings
4. **Operator Choice**: `_contains` on large arrays is slower than exact matches

## Error Handling

- Invalid operators default to auto-detection
- Missing fields cause condition to fail (false)
- Empty expressions match all documents
- Unknown operators log warnings and default to OR logic

This powerful filtering system gives you fine-grained control over document matching and relevance scoring, enabling
sophisticated search experiences similar to enterprise search platforms.
