# Search-Time Settings vs Core Settings

## Overview

The Go Search Engine has two distinct types of settings that behave very differently when updated:

- **🔍 Search-Time Settings**: Applied during search queries - **no reindexing needed**
- **🏗️ Core Settings**: Affect index structure - **full reindexing required**

## 🔍 Search-Time Settings

These settings only affect **how searches are performed**, not how documents are indexed. They can be updated **instantly** without rebuilding the index.

### Typo Tolerance Settings

```json
{
  "min_word_size_for_1_typo": 4, // Words ≥4 chars allow 1 typo
  "min_word_size_for_2_typos": 7 // Words ≥7 chars allow 2 typos
}
```

**What they do**: Control when typo tolerance kicks in during search
**Why instant**: Only affects search algorithm behavior, not indexed data

### Field-Level Search Behavior

```json
{
  "fields_without_prefix_search": ["id", "isbn"], // Disable prefix matching
  "no_typo_tolerance_fields": ["category", "status"], // Disable typos
  "distinct_field": "title" // Deduplicate by field
}
```

**What they do**: Control search behavior per field
**Why instant**: Only affects how search processes queries, not the index structure

## 🏗️ Core Settings

These settings affect **what gets indexed and how**, requiring a complete rebuild of the index.

### Index Structure Settings

```json
{
  "searchable_fields": ["title", "description"], // Which fields to index for search
  "filterable_fields": ["year", "genre", "rating"], // Which fields to index for filters
  "ranking_criteria": [
    // How to order results
    { "field": "rating", "order": "desc" },
    { "field": "year", "order": "desc" }
  ]
}
```

**What they do**: Define the fundamental structure of the search index
**Why reindexing needed**: Changes what data gets indexed and how it's stored

## ⚡ Performance Impact

| Setting Type    | Update Time   | API Response | Reindexing |
| --------------- | ------------- | ------------ | ---------- |
| **Search-Time** | < 1 second    | 202 Accepted | ❌ No      |
| **Core**        | Minutes/Hours | 202 Accepted | ✅ Yes     |

## 🔧 How It Works

### Search-Time Settings Update Process:

1. **Update settings in memory** (instant)
2. **Rebuild search service** with new settings (rebuilds typo finder, etc.)
3. **Persist settings to disk** (saves changes)
4. **✅ Done** - no document processing needed!

### Core Settings Update Process:

1. **Extract all documents** from index
2. **Clear the index** completely
3. **Update settings** in memory
4. **Rebuild search/indexing services**
5. **Reindex all documents** with new structure
6. **Persist everything** to disk

## 🚀 API Usage

### Updating Search-Time Settings (Fast)

```bash
curl -X PATCH http://localhost:8080/indexes/movies/settings \
  -H "Content-Type: application/json" \
  -d '{
    "min_word_size_for_1_typo": 3,
    "no_typo_tolerance_fields": ["genre"]
  }'

# Response: HTTP 202 (< 100ms)
{
  "status": "accepted",
  "message": "Settings update started for index 'movies' (search-time settings update)",
  "job_id": "job_12345",
  "reindexing_required": false  // ← No reindexing!
}
```

### Updating Core Settings (Slow)

```bash
curl -X PATCH http://localhost:8080/indexes/movies/settings \
  -H "Content-Type: application/json" \
  -d '{
    "searchable_fields": ["title", "description", "cast"],
    "filterable_fields": ["year", "genre", "rating", "language"]
  }'

# Response: HTTP 202 (< 100ms, but job takes longer)
{
  "status": "accepted",
  "message": "Settings update started for index 'movies' (full reindexing required)",
  "job_id": "job_67890",
  "reindexing_required": true   // ← Full reindexing needed!
}
```

## 🧠 Intelligent Detection

The engine **automatically detects** which type of update is needed:

```go
// Engine automatically determines the update type
requiresFullReindex := e.requiresFullReindexing(oldSettings, newSettings)

if requiresFullReindex {
    // Core settings changed - full reindexing needed
    return e.executeReindexJob(...)
} else {
    // Only search-time settings changed - instant update
    return e.executeSearchTimeSettingsUpdateJob(...)
}
```

## 💡 Best Practices

### ✅ Do This:

- **Tune typo tolerance** frequently to optimize search quality
- **Adjust field-level behaviors** based on user feedback
- **Update search-time settings** during business hours (they're instant)

### ⚠️ Be Careful:

- **Plan core setting changes** during maintenance windows
- **Test core changes** on staging first (they trigger full reindexing)
- **Monitor job progress** for core setting updates

## 🎯 Summary

| Aspect           | Search-Time Settings       | Core Settings              |
| ---------------- | -------------------------- | -------------------------- |
| **Purpose**      | Control search behavior    | Define index structure     |
| **Update Speed** | Instant (< 1 second)       | Slow (minutes to hours)    |
| **Reindexing**   | Not required               | Required                   |
| **When to Use**  | Tuning search quality      | Changing what's searchable |
| **Risk Level**   | Low (reversible instantly) | High (expensive to change) |

**Key Insight**: Search-time settings let you **tune search behavior** without the cost of reindexing, making the search engine much more **production-friendly** and **responsive to user needs**! 🎉
