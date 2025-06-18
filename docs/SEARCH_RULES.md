# 🎯 Search Rules Engine

The Go Search Engine includes a powerful **rules engine** that allows you to customize search results based on specific conditions. Rules can **pin documents to specific positions**, **hide documents**, or apply other modifications to search results automatically.

## 🏗️ Architecture Overview

The rules engine follows a **persistent, shared storage pattern** similar to the analytics system:

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   API Handlers  │    │  Search Handlers │    │  Rule Engine    │
│                 │    │                  │    │                 │
│ • Create Rules  │    │ • Apply Rules    │    │ • Evaluate      │
│ • Update Rules  │    │ • Add Rule Info  │    │ • Execute       │
│ • Delete Rules  │    │ • Return Results │    │ • Modify Results│
└─────────┬───────┘    └─────────┬────────┘    └─────────┬───────┘
          │                      │                       │
          └──────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────▼──────────────┐
                    │     FileRuleStore          │
                    │                            │
                    │ • Persistent JSON storage  │
                    │ • Thread-safe operations  │
                    │ • Automatic backup/restore │
                    │ • File: search_data/rules.json │
                    └────────────────────────────┘
```

### Key Components

- **🏪 FileRuleStore**: Persistent JSON-based storage in `search_data/rules.json`
- **⚙️ Rule Engine**: Evaluates conditions and applies actions to search results
- **🔗 Shared Instance**: Single rule store shared between API and search handlers
- **📊 Rule Information**: Descriptive rule application details in search responses

## 📋 Rule Structure

### Rule Components

```json
{
  "id": "uuid-string",
  "name": "Human-readable rule name",
  "index_name": "target-index-name", // or "*" for all indexes
  "is_active": true,
  "priority": 100, // Higher numbers = higher priority
  "conditions": [
    {
      "type": "query|result_count",
      "operator": "equals|contains|starts_with|ends_with|gt|gte|lt|lte",
      "value": "condition-value"
    }
  ],
  "actions": [
    {
      "type": "pin|hide",
      "target": {
        "type": "document_id|all_results",
        "operator": "equals|contains",
        "value": "target-value"
      },
      "position": 1 // Required for pin actions
    }
  ]
}
```

### Condition Types

| Type           | Description                           | Operators Available                              | Example                                                  |
| -------------- | ------------------------------------- | ------------------------------------------------ | -------------------------------------------------------- |
| `query`        | Query matching with various operators | `equals`, `contains`, `starts_with`, `ends_with` | `"office"` with `contains` operator matches "The Office" |
| `result_count` | Number of search results              | `equals`, `gt`, `gte`, `lt`, `lte`               | `> 10`, `<= 5`                                           |

### Action Types

| Type   | Description                       | Parameters            |
| ------ | --------------------------------- | --------------------- |
| `pin`  | Pin document to specific position | `position` (required) |
| `hide` | Hide document from results        | None                  |

## 🚀 Usage Examples

### Example 1: Pin Specific Content

Pin "The Office: Superfan Episodes" to position 1 when users search for "The Office":

```json
{
  "name": "Pin The Office Superfan Episodes",
  "index_name": "suggest-index",
  "is_active": true,
  "priority": 100,
  "conditions": [
    {
      "type": "query",
      "operator": "equals",
      "value": "The Office"
    }
  ],
  "actions": [
    {
      "type": "pin",
      "target": {
        "type": "document_id",
        "operator": "equals",
        "value": "23172c5c-ab8d-3473-bedc-d83459536b75_en-US_title"
      },
      "position": 1
    }
  ]
}
```

### Example 2: Hide Low-Quality Results

Hide documents when there are too many results (reduce noise):

```json
{
  "name": "Hide Low Priority Content",
  "index_name": "*",
  "is_active": true,
  "priority": 50,
  "conditions": [
    {
      "type": "result_count",
      "operator": "gt",
      "value": 50
    }
  ],
  "actions": [
    {
      "type": "hide",
      "target": {
        "type": "document_id",
        "operator": "contains",
        "value": "low-priority"
      }
    }
  ]
}
```

### Example 3: Global Rule

Apply to all indexes using `"index_name": "*"`:

```json
{
  "name": "Global Documentary Promotion",
  "index_name": "*",
  "is_active": true,
  "priority": 75,
  "conditions": [
    {
      "type": "query",
      "operator": "contains",
      "value": "documentary"
    }
  ],
  "actions": [
    {
      "type": "pin",
      "target": {
        "type": "document_id",
        "operator": "equals",
        "value": "featured-documentary-id"
      },
      "position": 1
    }
  ]
}
```

## 🔧 API Endpoints

### Create Rule

```bash
POST /api/v1/rules
Content-Type: application/json

{
  "name": "Rule Name",
  "index_name": "target-index",
  "is_active": true,
  "priority": 100,
  "conditions": [...],
  "actions": [...]
}
```

### List Rules

```bash
GET /api/v1/rules
GET /api/v1/rules?index_name=movies
GET /api/v1/rules?is_active=true
```

### Update Rule

```bash
PUT /api/v1/rules/{ruleId}
Content-Type: application/json

{
  "name": "Updated Rule Name",
  ...
}
```

### Delete Rule

```bash
DELETE /api/v1/rules/{ruleId}
```

### Toggle Rule Status

```bash
POST /api/v1/rules/{ruleId}/toggle
```

### Test Rule (Without Persisting)

```bash
POST /api/v1/rules/test
Content-Type: application/json

{
  "rule": {...},
  "context": {
    "query": "test query",
    "index_name": "test-index",
    "result_count": 10
  },
  "results": [...]
}
```

## 📊 Search Response Integration

When rules are applied to search results, the response includes detailed information:

```json
{
  "hits": [...],
  "total": 25,
  "page": 1,
  "page_size": 10,
  "took": 15,
  "rules": {
    "applied": true,
    "details": [
      {
        "rule_name": "Pin The Office Superfan Episodes",
        "action": "pinned to position 1",
        "trigger": "query match: 'The Office'",
        "document_ids": ["23172c5c-ab8d-3473-bedc-d83459536b75_en-US_title"]
      }
    ]
  }
}
```

### Rule Information Fields

| Field          | Description                    | Example                              |
| -------------- | ------------------------------ | ------------------------------------ |
| `applied`      | Whether any rules were applied | `true`                               |
| `rule_name`    | Name of the applied rule       | `"Pin The Office Superfan Episodes"` |
| `action`       | Description of what was done   | `"pinned to position 1"`             |
| `trigger`      | What triggered the rule        | `"query match: 'The Office'"`        |
| `document_ids` | Affected document IDs          | `["doc-id-1", "doc-id-2"]`           |

## 💾 Persistent Storage

### File-Based Storage

Rules are automatically persisted to `search_data/rules.json`:

```json
[
  {
    "id": "uuid-1",
    "name": "Pin The Office Superfan Episodes",
    "index_name": "suggest-index",
    "is_active": true,
    "priority": 100,
    "conditions": [...],
    "actions": [...],
    "created_at": "2024-06-14T10:30:00Z",
    "updated_at": "2024-06-14T10:35:00Z"
  }
]
```

### Storage Features

- ✅ **Automatic Persistence**: All rule operations are immediately saved to disk
- ✅ **Crash Recovery**: Rules survive server restarts and crashes
- ✅ **Thread Safety**: Concurrent rule operations are safely handled
- ✅ **Rollback Support**: Failed operations automatically rollback in-memory changes
- ✅ **Directory Creation**: Storage directory is created automatically if missing

### Storage Location

```
search_data/
├── analytics.json      # Analytics data
├── rules.json         # Rules data (NEW)
└── suggest-index/     # Index data
    ├── documents.json
    ├── inverted_index.json
    └── settings.json
```

## ⚡ Performance Considerations

### Rule Evaluation Order

1. **Priority Sorting**: Rules are sorted by priority (higher first)
2. **Condition Evaluation**: All conditions must be met for rule to apply
3. **Action Application**: Actions are applied in the order they appear
4. **Position Conflicts**: Later rules cannot override earlier pin positions

### Optimization Tips

- **Use Specific Conditions**: More specific conditions evaluate faster
- **Set Appropriate Priorities**: Higher priority rules are evaluated first
- **Limit Global Rules**: Rules with `"index_name": "*"` apply to all searches
- **Monitor Performance**: Rule execution time is tracked and reported

## 🔍 Debugging and Monitoring

### Rule Execution Information

Every search response includes rule execution details when rules are applied:

```json
{
  "rules": {
    "applied": true,
    "details": [
      {
        "rule_name": "Pin Featured Content",
        "action": "pinned to position 1",
        "trigger": "query match: 'matrix'",
        "document_ids": ["featured-matrix-doc"]
      }
    ]
  }
}
```

### Common Issues

| Issue               | Cause                  | Solution                               |
| ------------------- | ---------------------- | -------------------------------------- |
| Rules not applying  | Rule store separation  | Ensure shared FileRuleStore instance   |
| Rules field is null | No rules matched       | Check rule conditions and index_name   |
| Performance issues  | Too many global rules  | Use index-specific rules when possible |
| Storage errors      | Permissions/disk space | Check file permissions and disk space  |

## 🧪 Testing

### Unit Tests

Rules engine includes comprehensive test coverage:

```bash
# Run rule engine tests
go test ./internal/rules/...

# Run API handler tests
go test ./api/...

# Run with coverage
go test -cover ./internal/rules/...
```

### Integration Testing

Test rules with live search queries:

```bash
# Create test rule
curl -X POST "http://localhost:8080/api/v1/rules" \
  -H "Content-Type: application/json" \
  -d '{"name": "Test Rule", ...}'

# Test search with rule
curl -X POST "http://localhost:8080/indexes/test-index/_search" \
  -H "Content-Type: application/json" \
  -d '{"query": "test query", "page": 1, "page_size": 10}'

# Verify rule information in response
```

## 🚀 Migration from Memory Store

If you have existing rules in memory store, they need to be migrated to the persistent store:

1. **Export existing rules** via `/api/v1/rules`
2. **Restart server** (clears memory store, loads FileRuleStore)
3. **Recreate rules** via POST `/api/v1/rules` for each rule
4. **Verify persistence** by restarting server and checking rules still exist

## 🎯 Best Practices

### Rule Design

- **Use descriptive names**: Make rule purpose clear
- **Set appropriate priorities**: Important rules should have higher priority
- **Test thoroughly**: Use `/api/v1/rules/test` endpoint before creating
- **Monitor impact**: Check rule application in search responses

### Performance

- **Index-specific rules**: Prefer specific `index_name` over `"*"`
- **Efficient conditions**: Use exact matches when possible
- **Limit rule count**: Too many rules can impact search performance
- **Regular cleanup**: Remove unused or ineffective rules

### Maintenance

- **Regular audits**: Review rule effectiveness periodically
- **Version control**: Consider backing up `search_data/rules.json`
- **Documentation**: Document business logic behind each rule
- **Testing**: Test rules after any search engine updates

---

The search rules engine provides powerful customization capabilities while maintaining high performance and reliability through persistent storage and comprehensive monitoring! 🎉
