# Pull Request

## 📋 Description

<!-- Provide a brief description of what this PR does -->

### What changed?

-

### Why was this change needed?

-

## 🔧 Type of Change

<!-- Mark the relevant option with an [x] -->

- [ ] 🐛 **Bug fix** (non-breaking change that fixes an issue)
- [ ] ✨ **New feature** (non-breaking change that adds functionality)
- [ ] 💥 **Breaking change** (fix or feature that would cause existing functionality to not work as expected)
- [ ] 📚 **Documentation** (changes to documentation only)
- [ ] 🧹 **Code cleanup** (refactoring, formatting, removing unused code)
- [ ] ⚡ **Performance** (changes that improve performance)
- [ ] 🔒 **Security** (changes that address security vulnerabilities)
- [ ] 🧪 **Tests** (adding or updating tests)

## 🎯 Areas Affected

<!-- Mark all that apply with [x] -->

- [ ] **Search Engine Core** (`internal/engine/`)
- [ ] **Search Service** (`internal/search/`)
- [ ] **Indexing Service** (`internal/indexing/`)
- [ ] **Typo Tolerance** (`internal/typoutil/`)
- [ ] **API Handlers** (`api/`)
- [ ] **Analytics** (`internal/analytics/`)
- [ ] **Job Management** (`internal/jobs/`)
- [ ] **Configuration** (`config/`)
- [ ] **Documentation** (`docs/`)
- [ ] **Examples** (`examples/`)

## 🧪 Testing

<!-- Describe how you tested your changes -->

### Test Coverage

- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing performed
- [ ] Performance testing performed (if applicable)

### Test Commands Run

```bash
# List the commands you ran to test your changes
go test ./...
go build ./cmd/search_engine/
```

### Test Results

<!-- Describe the test results or paste relevant output -->

## 📊 Performance Impact

<!-- If this change affects performance, describe the impact -->

- [ ] No performance impact expected
- [ ] Performance improvement (describe below)
- [ ] Performance regression (justify below)
- [ ] Performance impact unknown/needs measurement

**Details:**

<!-- Describe performance changes, include benchmarks if available -->

## 🔄 Breaking Changes

<!-- If this is a breaking change, describe what breaks and how to migrate -->

- [ ] No breaking changes
- [ ] Breaking changes (describe below)

**Migration Guide:**

<!-- If breaking changes, provide migration instructions -->

## 📝 Checklist

<!-- Ensure all items are completed before requesting review -->

### Code Quality

- [ ] Code follows the project's coding standards
- [ ] Self-review of code completed
- [ ] Code is properly commented (clarifies logic, not implementation history)
- [ ] No debugging code or console logs left in
- [ ] Error handling is appropriate

### Documentation

- [ ] Updated relevant documentation in `docs/`
- [ ] Updated API spec (`api-spec.yaml`) if API changes
- [ ] Updated README if needed
- [ ] Added/updated code comments where necessary

### Testing & Validation

- [ ] All tests pass locally
- [ ] New tests added for new functionality
- [ ] Edge cases considered and tested
- [ ] Manual testing completed
- [ ] No linter warnings or errors

### Dependencies & Compatibility

- [ ] No new dependencies added (or justified if added)
- [ ] Backward compatibility maintained (or breaking changes documented)
- [ ] Changes work with existing indexes and data

## 🔗 Related Issues

<!-- Link to related issues -->

Closes #<!-- issue number -->
Related to #<!-- issue number -->

## 📸 Screenshots/Examples

<!-- If applicable, add screenshots or examples of the changes -->

## 🤔 Questions for Reviewers

<!-- Any specific areas you'd like reviewers to focus on -->

-
-

## 📋 Deployment Notes

<!-- Any special considerations for deployment -->

- [ ] No special deployment considerations
- [ ] Requires data migration (describe below)
- [ ] Requires configuration changes (describe below)
- [ ] Requires index rebuilding (describe below)

**Special Instructions:**

<!-- Describe any special deployment steps -->

---

## 🏷️ Labels

<!-- Suggested labels for this PR -->
<!-- The maintainer will apply these -->

**Suggested labels:** `enhancement`, `bug`, `documentation`, `performance`, `breaking-change`, `needs-review`
