📚 **[Documentation Index](./README.md)** | [🏠 Project Home](../README.md)

---

# Field Naming Guidelines

## ⚠️ Underscore Operator Conflicts

### The Problem

The search engine uses underscores (`_`) to separate field names from filter operators. This can create parsing conflicts if your field names end with operator keywords.

### Examples of Problematic Field Names

❌ **Avoid these field names:**

```json
{
  "searchable_fields": ["user_exact", "description_contains"],
  "filterable_fields": ["rating_gte", "status_ne", "date_lt"]
}
```

**Why they're problematic:**

- `user_exact` → Gets parsed as field `user` with operator `_exact`
- `description_contains` → Gets parsed as field `description` with operator `_contains`
- `rating_gte` → Gets parsed as field `rating` with operator `_gte`

### Filter Operators to Avoid as Field Name Suffixes

| Operator           | Purpose                | Don't end field names with |
| ------------------ | ---------------------- | -------------------------- |
| `_exact`           | Exact match            | `*_exact`                  |
| `_ne`              | Not equal              | `*_ne`                     |
| `_gt`              | Greater than           | `*_gt`                     |
| `_gte`             | Greater than or equal  | `*_gte`                    |
| `_lt`              | Less than              | `*_lt`                     |
| `_lte`             | Less than or equal     | `*_lte`                    |
| `_contains`        | Contains substring     | `*_contains`               |
| `_ncontains`       | Does not contain       | `*_ncontains`              |
| `_contains_any_of` | Contains any of values | `*_contains_any_of`        |

## ✅ Recommended Field Naming Patterns

### Good Field Names

```json
{
  "searchable_fields": ["title", "description", "content", "author_name"],
  "filterable_fields": [
    "year",
    "rating",
    "popularity",
    "release_date",
    "genre_list"
  ]
}
```

### Safe Underscore Usage

- `release_date` ✅ (doesn't end with operator)
- `author_name` ✅ (doesn't end with operator)
- `user_id` ✅ (doesn't end with operator)
- `page_count` ✅ (doesn't end with operator)

## 🔧 Validation

You can validate your field names using the validation function:

```go
settings := &config.IndexSettings{
    SearchableFields: []string{"title", "user_exact"}, // This will show a conflict
    FilterableFields: []string{"year", "rating"},
}

conflicts := settings.ValidateFieldNames()
if len(conflicts) > 0 {
    for _, conflict := range conflicts {
        fmt.Println("Warning:", conflict)
    }
}
```

## 🛠️ What if I Already Have Conflicting Field Names?

### Option 1: Rename Fields (Recommended)

```json
// Before (problematic)
"filterable_fields": ["rating_exact", "user_contains"]

// After (fixed)
"filterable_fields": ["exact_rating", "user_info"]
```

### Option 2: Use Alternative Naming

```json
// Instead of "description_contains"
"filterable_fields": ["description_text", "description_content"]

// Instead of "rating_gte"
"filterable_fields": ["min_rating", "rating_threshold"]
```

## 📝 Current System Behavior

If you have conflicting field names, the current system will:

1. Always interpret the suffix as an operator if it matches a known operator
2. You cannot filter fields that end with operator names using exact match
3. No error is thrown - the filter just won't work as expected

## 🚀 Future Improvements

Potential solutions being considered:

1. **Validation on index creation** - Reject field names with conflicts
2. **Alternative operator syntax** - Use `:` instead of `_` (e.g., `year:gte`)
3. **Explicit operator requirement** - Require explicit `_exact` for all exact matches

## 💡 Best Practices Summary

1. ✅ Use descriptive field names without operator suffixes
2. ✅ Validate field names before creating indexes
3. ✅ Use underscores for word separation (e.g., `release_date`)
4. ❌ Don't end field names with filter operators
5. ❌ Don't use field names like `field_exact`, `name_contains`, etc.
