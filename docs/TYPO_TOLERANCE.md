📚 **[Documentation Index](./README.md)** | [🏠 Project Home](../README.md)

---

# Typo Tolerance System

## Overview

The Go Search Engine features a typo tolerance system that automatically corrects common typing errors and misspellings in search queries. The system uses the **Damerau-Levenshtein distance algorithm** to detect and match variations of words, providing a seamless search experience even when users make mistakes.

## How It Works

### Core Algorithm: Damerau-Levenshtein Distance

The typo tolerance system uses Damerau-Levenshtein distance, which measures the minimum number of single-character edits needed to transform one word into another. It supports four types of edits:

1. **Substitution**: `cat` → `bat` (replace 'c' with 'b')
2. **Insertion**: `cat` → `cart` (insert 'r')
3. **Deletion**: `cart` → `cat` (delete 'r')
4. **Transposition**: `form` → `from` (swap 'r' and 'o')

### Distance Levels

The system supports two levels of typo tolerance:

- **1-typo tolerance**: Words with 1 character difference (e.g., "matrix" matches "matix")
- **2-typo tolerance**: Words with 2 character differences (e.g., "matrix" matches "matrx")

## Configuration

### Index-Level Settings

Configure typo tolerance when creating or updating an index:

```json
{
  "name": "movies",
  "settings": {
    "min_word_size_for_1_typo": 4, // Words ≥4 chars allow 1 typo
    "min_word_size_for_2_typos": 7, // Words ≥7 chars allow 2 typos
    "non_typo_tolerant_words": ["id", "isbn", "sku"] // Exact match only
  }
}
```

### Query-Level Overrides

Override settings for specific searches:

```bash
curl -X POST http://localhost:8080/indexes/movies/_search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "matrix",
    "min_word_size_for_1_typo": 3,
    "min_word_size_for_2_typos": 6
  }'
```

## Smart Features

### 1. Redundant Match Prevention

The system automatically prevents confusing duplicate results by showing only the best quality typo match per query token:

**Example:**

```
Query: "steve careel"
Document: "Steve Carell"

Without prevention: steve, carel(typo), carell(typo) (confusing)
With prevention:    steve, carell(typo) (clean)
```

### 2. Performance Optimization

#### Dual Stopping Criteria

- **Result limit**: Stop after finding 500 potential matches
- **Time limit**: Stop after 50ms to maintain responsiveness
- **First criterion met wins**: Ensures consistent performance

#### Early Termination

- Skip words where length difference exceeds maximum distance
- Stop calculations when minimum distance exceeds threshold
- Pre-filter based on word length before expensive calculations

### 3. Intelligent Caching

The system caches typo calculations for frequently searched terms:

- **Cache size**: Limited to 1,000 entries to prevent memory bloat
- **Thread-safe**: Concurrent access with read-write locks
- **Performance**: Up to 95,000x faster for cached results

## Usage Examples

### Basic Typo Tolerance

```bash
# Search with automatic typo correction
curl -X POST http://localhost:8080/indexes/movies/_search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "matirx reloaded",  // "matrix" and "reloaded" with typos
    "page_size": 10
  }'
```

**Response shows typo matches:**

```json
{
  "hits": [
    {
      "document": {
        "title": "The Matrix Reloaded",
        "year": 2003
      },
      "field_matches": {
        "title": ["matrix(typo)", "reloaded"]
      },
      "hit_info": {
        "num_typos": 1,
        "number_exact_words": 1
      }
    }
  ]
}
```

### Field-Specific Configuration

```bash
# Disable typos for specific fields
curl -X POST http://localhost:8080/indexes/products/_search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "SKU12345",  // Exact match required for SKU
    "restrict_searchable_fields": ["sku", "title"]
  }'
```

### Adjusting Sensitivity

```bash
# More aggressive typo tolerance
curl -X POST http://localhost:8080/indexes/books/_search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "harrypotter",
    "min_word_size_for_1_typo": 3,  // Allow typos in shorter words
    "min_word_size_for_2_typos": 5   // Allow 2 typos in shorter words
  }'
```

## Performance Characteristics

### Benchmarks

| Scenario                  | Performance | Notes                           |
| ------------------------- | ----------- | ------------------------------- |
| Cached lookups            | ~0.0001ms   | 95,000x faster than calculation |
| Fresh calculations        | ~1.3ms      | With early termination          |
| Large indexes (50K terms) | <50ms       | Time limit prevents slowdown    |

### Memory Usage

- **Cache memory**: ~100KB maximum (1,000 entries × ~100 bytes)
- **Algorithm memory**: O(n) space complexity (3-row algorithm)
- **Search memory**: Minimal overhead per query

### Scaling Behavior

The system maintains consistent performance across index sizes:

- **Small indexes (1K terms)**: Sub-millisecond typo processing
- **Medium indexes (10K terms)**: ~1-5ms typo processing
- **Large indexes (100K+ terms)**: 50ms maximum (time limit)

## Best Practices

### Configuration Guidelines

1. **Word Size Thresholds**

   - Use `min_word_size_for_1_typo: 4` for most use cases
   - Increase to 5-6 for technical/scientific content
   - Decrease to 3 for casual/social content

2. **Non-Typo Tolerant Words**

   - Always include: IDs, SKUs, codes, technical identifiers
   - Consider: Brand names, proper nouns, technical terms

3. **Performance Tuning**
   - Monitor search times in production
   - Adjust time limits for your specific needs
   - Consider index size when setting thresholds

### Query Optimization

1. **Field Restriction**

   - Use `restrict_searchable_fields` to limit typo scope
   - Combine with exact match fields for hybrid searches

2. **Query Structure**

   - Break complex queries into multiple terms
   - Use filters to reduce typo search space

3. **User Experience**
   - Show typo indicators in results: `matrix(typo)`
   - Provide "did you mean" suggestions
   - Allow users to see exact vs typo matches

## Troubleshooting

### Common Issues

**Too many false positives:**

- Increase `min_word_size_for_1_typo`
- Add problematic terms to `non_typo_tolerant_words`
- Use field restriction to limit scope

**Missing expected matches:**

- Decrease word size thresholds
- Check if terms are in `non_typo_tolerant_words`
- Verify index configuration

**Slow search performance:**

- Monitor time limit warnings in logs
- Consider increasing time limits for specific queries
- Optimize index size and structure

### Debug Information

Enable detailed logging to see typo processing:

```bash
# Check for time limit warnings
tail -f server.log | grep "Typo search time limit"

# Example warning:
# Warning: Typo search time limit reached (50.0ms) - found 123/500 tokens, 15432 terms remaining unchecked (term='action', distance=1)
```

## Implementation Details

### Algorithm Complexity

- **Time complexity**: O(m×n) where m,n are word lengths
- **Space complexity**: O(n) with 3-row optimization
- **Early termination**: Reduces average case significantly

### Thread Safety

The typo system is fully thread-safe:

- Concurrent searches supported
- Cache access protected with read-write locks
- No shared mutable state between requests

### Integration Points

The typo tolerance integrates with:

- **Search engine**: Automatic application during queries
- **Indexing**: Updates cached terms when documents added
- **Analytics**: Tracks typo usage and performance
- **API**: Exposes configuration and override options

## Related Documentation

- **[Search Features](./SEARCH_FEATURES.md)** - Complete search capabilities overview
- **[Performance Guide](./SEARCH_TIME_SETTINGS.md)** - Optimizing search performance
- **[API Reference](../api-spec.yaml)** - Complete API documentation
- **[Analytics](./ANALYTICS.md)** - Monitoring typo usage and performance
