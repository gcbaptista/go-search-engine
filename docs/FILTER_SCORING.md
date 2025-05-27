# Filter Scoring

Filter scoring allows you to assign scores to documents based on which filters they match. This enables you to boost the
relevance of documents that match specific criteria, giving you fine-grained control over search result ranking.

> **New**: Advanced filter expressions with AND/OR logic are now available!
> See [Filter Expressions](FILTER_EXPRESSIONS.md) for complex boolean filtering inspired by Algolia.

## How It Works

1. **FilterScoring Configuration**: In your search query, specify a `filter_scoring` map that assigns scores to filter
   keys
2. **Score Calculation**: When a document matches a filter, the corresponding score is added to the document's filter
   score
3. **Ranking Integration**: Use the special `~filters` ranking criterion to sort results by filter score

## Basic Example

```json
{
  "query": "action movie",
  "filters": {
    "operator": "AND",
    "filters": [
      {
        "field": "genre",
        "operator": "_exact",
        "value": "Action",
        "score": 1.0
      },
      {
        "field": "is_premium",
        "operator": "_exact",
        "value": true,
        "score": 2.5
      },
      { "field": "year", "operator": "_gte", "value": 2020, "score": 0.5 }
    ]
  }
}
```

In this example:

- Documents matching `genre: Action` get +1.0 filter score
- Documents matching `is_premium: true` get +2.5 filter score
- Documents matching `year >= 2020` get +0.5 filter score
- A document matching all three filters would have a total filter score of 4.0

## Ranking with Filter Scores

Configure your index to use filter scores in ranking:

```json
{
  "name": "movies",
  "searchable_fields": ["title", "description"],
  "filterable_fields": ["genre", "year", "is_premium", "rating"],
  "ranking_criteria": [
    { "field": "~filters", "order": "desc" },
    { "field": "~score", "order": "desc" },
    { "field": "popularity", "order": "desc" }
  ]
}
```

This ranking configuration:

1. **Primary**: Sort by filter score (highest first)
2. **Secondary**: Sort by search relevance score (highest first)
3. **Tertiary**: Sort by popularity field (highest first)

## Special Ranking Fields

- `~filters`: Uses the calculated filter score
- `~score`: Uses the search relevance score

## Advanced Example

```json
{
  "query": "thriller",
  "filters": {
    "operator": "AND",
    "filters": [
      {
        "field": "genre",
        "operator": "_contains",
        "value": "Thriller",
        "score": 2.0
      },
      { "field": "rating", "operator": "_gte", "value": 7.0, "score": 1.5 },
      { "field": "year", "operator": "_gte", "value": 2015, "score": 0.8 },
      {
        "field": "is_premium",
        "operator": "_exact",
        "value": true,
        "score": 3.0
      }
    ]
  }
}
```

This would prioritize:

1. Premium content (+3.0)
2. Thriller genre (+2.0)
3. High-rated content (+1.5)
4. Recent content (+0.8)

## Response Format

The filter score is included in the hit info:

```json
{
  "hits": [
    {
      "document": {
        "documentID": "movie_123",
        "title": "Premium Thriller",
        "genre": "Thriller",
        "rating": 8.5,
        "year": 2022,
        "is_premium": true
      },
      "score": 2.5,
      "field_matches": {
        "title": ["thriller"]
      },
      "hit_info": {
        "num_typos": 0,
        "number_exact_words": 1,
        "filter_score": 7.3
      }
    }
  ]
}
```

## Use Cases

### Content Boosting

Boost premium or featured content:

```json
"filters": {
  "operator": "OR",
  "filters": [
    { "field": "is_premium", "operator": "_exact", "value": true, "score": 5.0 },
    { "field": "is_featured", "operator": "_exact", "value": true, "score": 3.0 }
  ]
}
```

### Recency Boosting

Favor newer content:

```json
"filters": {
  "operator": "OR",
  "filters": [
    { "field": "year", "operator": "_gte", "value": 2020, "score": 1.0 },
    { "field": "created_recently", "operator": "_exact", "value": true, "score": 2.0 }
  ]
}
```

### Quality Boosting

Prioritize high-quality content:

```json
"filters": {
  "operator": "OR",
  "filters": [
    { "field": "rating", "operator": "_gte", "value": 8.0, "score": 1.5 },
    { "field": "verified", "operator": "_exact", "value": true, "score": 2.0 },
    { "field": "editor_choice", "operator": "_exact", "value": true, "score": 3.0 }
  ]
}
```

### Geographic Relevance

Boost local content:

```json
"filters": {
  "operator": "OR",
  "filters": [
    { "field": "country", "operator": "_exact", "value": "US", "score": 2.0 },
    { "field": "region", "operator": "_exact", "value": "California", "score": 1.0 },
    { "field": "city", "operator": "_exact", "value": "San Francisco", "score": 3.0 }
  ]
}
```

## Important Notes

1. **Filter Requirement**: Only filters that are actually applied (in the `filters` object) can contribute to the filter score
2. **Score Integration**: Filter scores are specified directly in each filter condition using the `score` property
3. **Logical Operators**: Use `AND` for all-or-nothing scoring, `OR` for additive scoring across different conditions
4. **Optional Scoring**: You can apply filters without scoring by omitting the `score` property from filter conditions

## Multi-Search Support

Filter scoring works with multi-search queries:

```json
{
  "queries": [
    {
      "name": "premium_search",
      "query": "action",
      "filters": {
        "operator": "AND",
        "filters": [
          {
            "field": "is_premium",
            "operator": "_exact",
            "value": true,
            "score": 5.0
          }
        ]
      }
    },
    {
      "name": "regular_search",
      "query": "action",
      "filters": {
        "operator": "AND",
        "filters": [
          { "field": "year", "operator": "_gte", "value": 2020, "score": 1.0 }
        ]
      }
    }
  ]
}
```

This feature gives you powerful control over search result relevance, allowing you to implement sophisticated ranking
strategies based on your business logic and user preferences.
