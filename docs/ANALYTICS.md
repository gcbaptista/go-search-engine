# Analytics Dashboard Implementation

## Overview

The Go Search Engine now includes a comprehensive analytics system that tracks search events and provides detailed dashboard data for monitoring search engine performance and usage patterns.

## Features

### 📊 Dashboard Metrics

The analytics dashboard provides the following key metrics:

- **Total Searches**: Number of searches performed in the last 24 hours
- **Search Performance**: Hourly breakdown of search activity and response times
- **Popular Searches**: Most frequently searched terms with trend indicators
- **Index Usage**: Document counts, search counts, and size metrics per index
- **Response Time Distribution**: Performance buckets (0-25ms, 25-50ms, 50-100ms, 100ms+)
- **Search Type Statistics**: Breakdown by search type (exact match, fuzzy, filtered, wildcard)
- **System Health**: Memory usage, CPU usage, disk space, and index health

### 🔍 Search Event Tracking

Every search request is automatically tracked with the following information:

- Index name
- Search query
- Search type (exact_match, fuzzy_search, filtered, wildcard)
- Response time
- Result count
- Applied filters
- Timestamp

## API Endpoint

### GET /analytics

Returns comprehensive analytics dashboard data.

**Response Example:**

```json
{
  "total_searches": 12847,
  "searches_change_percent": 12.5,
  "avg_response_time": 45,
  "response_time_change": "down",
  "total_documents": 1234,
  "documents_change_count": 156,
  "active_indexes": 8,
  "indexes_change_count": 2,
  "search_performance_24h": [
    {
      "hour": 0,
      "search_count": 45,
      "avg_response_time": 42
    }
  ],
  "popular_searches": [
    {
      "query": "matrix",
      "search_count": 1247,
      "trend_change": "up"
    }
  ],
  "index_usage": [
    {
      "index_name": "movies",
      "document_count": 1247,
      "search_count": 2341,
      "size_mb": 12.4
    }
  ],
  "response_time_distribution": {
    "bucket_0_25ms": 567,
    "bucket_25_50ms": 423,
    "bucket_50_100ms": 234,
    "bucket_100ms_plus": 156,
    "percentage_0_25": 45.0,
    "percentage_25_50": 35.0,
    "percentage_50_100": 15.0,
    "percentage_100_plus": 5.0
  },
  "search_types": {
    "exact_match": 567,
    "fuzzy_search": 423,
    "filtered": 234,
    "wildcard": 156
  },
  "system_health": {
    "memory_usage_percent": 68.0,
    "cpu_usage_percent": 23.0,
    "disk_space_percent": 45.0,
    "index_health_percent": 100.0
  }
}
```

## Implementation Details

### Architecture

The analytics system consists of:

1. **Analytics Service** (`internal/analytics/service.go`): Core analytics logic
2. **Analytics Models** (`model/analytics.go`): Data structures for analytics
3. **API Integration** (`api/handlers.go`): HTTP endpoint and search tracking
4. **Data Persistence**: Analytics data is stored in `search_data/analytics.json`

### Search Type Detection

The system automatically categorizes searches:

- **exact_match**: Simple text queries without wildcards or filters
- **fuzzy_search**: Single-word queries that may use typo tolerance
- **filtered**: Searches with applied filters or empty queries with filters
- **wildcard**: Queries containing `*` or `?` characters

### Data Retention

- Analytics events are limited to the last 10,000 events for performance
- Data is persisted asynchronously to avoid impacting search response times
- Historical data is used for trend calculations and change percentages

### Performance Considerations

- Search event tracking is performed asynchronously
- Analytics data loading is done at service startup
- Dashboard data calculation is performed on-demand
- Memory usage is controlled through event count limits

## Usage Examples

### Testing the Analytics Endpoint

```bash
# Get analytics dashboard data
curl -X GET http://localhost:8080/analytics | jq .

# Perform some searches to generate data
curl -X POST http://localhost:8080/indexes/movies/_search \
  -H "Content-Type: application/json" \
  -d '{"query": "matrix", "restrict_searchable_fields": ["title"]}'

# Check updated analytics
curl -X GET http://localhost:8080/analytics | jq '.total_searches, .popular_searches'
```

### Integration with Frontend

The analytics endpoint is designed to be consumed by dashboard frontends:

```javascript
// Fetch analytics data
const response = await fetch("/analytics");
const analytics = await response.json();

// Use the data for charts and metrics
console.log(`Total searches: ${analytics.total_searches}`);
console.log(`Popular searches:`, analytics.popular_searches);
```

## Future Enhancements

Potential improvements for the analytics system:

1. **Historical Tracking**: Store daily/weekly/monthly aggregates
2. **Real-time Updates**: WebSocket support for live dashboard updates
3. **Advanced Metrics**: Query complexity analysis, user session tracking
4. **Export Capabilities**: CSV/Excel export for analytics data
5. **Alerting**: Threshold-based alerts for performance issues
6. **Geographic Analytics**: Track search patterns by region
7. **A/B Testing**: Support for search algorithm experimentation

## Testing

The analytics system includes comprehensive tests:

```bash
# Run analytics tests
go test ./internal/analytics/

# Run all tests
go test ./...
```

## Configuration

Analytics behavior can be configured through constants in `internal/analytics/service.go`:

- `maxEventsToKeep`: Maximum number of events to retain (default: 10,000)
- `analyticsDataFile`: Path to analytics data file (default: "search_data/analytics.json")

## Monitoring

Monitor analytics system health through:

- Log messages for data persistence issues
- System health metrics in the dashboard
- Memory usage tracking
- Response time monitoring
