# CLAUDE.md - Go Search Engine Development Guide

📚 **[Documentation Index](./README.md)** | [🏠 Project Home](../README.md)

---

## Project Overview

**Go Search Engine** is a high-performance, full-text search engine built in Go with advanced features including typo tolerance, filtering, ranking, and prefix search capabilities. The project implements a modular, RESTful architecture designed for scalability and maintainability.

### Key Features

- Full-text search with configurable typo tolerance (Levenshtein distance)
- Query-time typo tolerance override (customize minWordSizes per search request)
- Prefix search and autocomplete capabilities
- Advanced filtering with multiple operators (exact, range, contains, etc.)
- Flexible ranking with custom criteria and sort orders
- Document deduplication and Unicode support
- Schema-agnostic JSON document handling
- RESTful API with comprehensive OpenAPI 3.0 specification
- Optimized inverted index data structures for high performance

### Primary Technologies

- **Language**: Go 1.24.2+
- **Web Framework**: Gin (v1.10.0)
- **API Documentation**: OpenAPI 3.0 specification
- **Testing**: Go's built-in testing framework
- **UUID Generation**: google/uuid (v1.6.0)
- **Data Persistence**: Custom file-based storage in `search_data/` directory

## Coding Conventions

### Code Style

- **Indentation**: Use tabs (standard Go convention)
- **Line Length**: Aim for 120 characters maximum
- **Formatting**: Always use `gofmt` to format code
- **Imports**: Group imports (standard library, third-party, local packages)

### Naming Conventions

- **Packages**: Use lowercase, single words when possible (`search`, `tokenizer`, `typoutil`)
- **Functions**: Use camelCase (`NewEngine`, `SearchDocuments`)
- **Variables**: Use camelCase for local variables, avoid abbreviations
- **Constants**: Use UPPER_SNAKE_CASE for constants
- **Structs**: Use PascalCase (`IndexSettings`, `SearchRequest`)

### Field Naming Guidelines

⚠️ **Critical**: Avoid field names ending with filter operators to prevent parsing conflicts:

**Avoid these suffixes:**

- `*_exact`, `*_ne`, `*_gt`, `*_gte`, `*_lt`, `*_lte`
- `*_contains`, `*_ncontains`, `*_contains_any_of`

**Good field names:**

```go
// ✅ Recommended
[]string{"title", "description", "author_name", "release_date", "user_id"}

// ❌ Avoid
[]string{"user_exact", "rating_gte", "description_contains"}
```

### Directory Structure

Follow the established modular architecture:

```
├── .github/                # GitHub workflows and templates
│   ├── workflows/         # CI/CD workflows
│   └── pull_request_template.md
├── api/                    # HTTP handlers and routing (modularized)
│   ├── analytics_handlers.go    # Analytics endpoints
│   ├── document_handlers.go     # Document management endpoints
│   ├── handlers.go              # Core API setup and middleware
│   ├── index_handlers.go        # Index management endpoints
│   ├── job_handlers.go          # Job management endpoints
│   └── search_handlers.go       # Search endpoints
├── cmd/search_engine/      # Main application entry point
├── config/                 # Configuration structures
├── docs/                   # Project documentation
├── index/                  # Inverted index implementation
├── internal/               # Private application code
│   ├── analytics/         # Analytics and monitoring
│   ├── engine/            # Core engine orchestration (modularized)
│   │   ├── async_operations.go  # Asynchronous operations
│   │   ├── engine.go            # Main engine logic
│   │   ├── index_management.go  # Index lifecycle management
│   │   ├── instance.go          # Index instance management
│   │   ├── persistence.go       # Data persistence operations
│   │   └── settings_management.go # Settings and configuration
│   ├── indexing/          # Document indexing service
│   ├── jobs/              # Background job management
│   ├── persistence/       # Data persistence layer
│   ├── search/            # Search service implementation (modularized)
│   │   ├── filtering.go         # Document filtering and deduplication
│   │   ├── multi_search.go      # Parallel multi-search functionality
│   │   └── service.go           # Core search logic
│   ├── tokenizer/         # Text tokenization
│   └── typoutil/          # Typo tolerance utilities
├── model/                 # Data models and structures
├── scripts/               # Utility scripts
├── search_data/           # Data storage directory
├── services/              # Service interfaces
└── store/                 # Document storage implementation
```

### Error Handling

- Always return errors as the last return value
- Use descriptive error messages with context
- Wrap errors with additional context using `fmt.Errorf`
- Handle errors at the appropriate level (don't ignore them)

### Documentation

- Document all public functions, types, and constants
- Use complete sentences in comments
- Include examples for complex functionality
- Follow Go documentation conventions (start with the item name)

## Linting Rules

### Required Tools

- **gofmt**: Code formatting (use `gofmt -w .` to format all files)
- **go vet**: Static analysis for common mistakes and potential bugs
- **golangci-lint**: Comprehensive meta-linter that runs multiple linters (primary tool)
- **gosec**: Security-focused static analysis scanner

### Installation

Install the required linting tools:

```bash
# Install golangci-lint (recommended method)
# On macOS with Homebrew:
brew install golangci-lint

# On Linux/macOS with curl:
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Verify installation
golangci-lint version
```

### Pre-commit Checks

Run these commands before committing:

```bash
# Format all Go files
gofmt -w .

# Check for formatting issues
gofmt -l .

# Run static analysis
go vet ./...

# Run comprehensive linting
golangci-lint run

# Run security scanner
gosec ./...

# Run all tests
go test ./...

# Build the application
go build ./cmd/search_engine
```

### Post-Major Task Requirements

**MANDATORY**: After completing any major task (feature implementation, significant refactoring, API changes), run the complete linting suite:

```bash
# Complete linting workflow
echo "🔍 Running complete linting suite..."

# 1. Format code
gofmt -w .
echo "✅ Code formatted"

# 2. Check for formatting issues
if [ "$(gofmt -l .)" ]; then
    echo "❌ Formatting issues found"
    gofmt -l .
    exit 1
fi

# 3. Run static analysis
go vet ./...
echo "✅ Static analysis passed"

# 4. Run comprehensive linting
golangci-lint run --timeout=5m
echo "✅ Linting passed"

# 5. Run security scan
gosec ./...
echo "✅ Security scan passed"

# 6. Run tests
go test ./...
echo "✅ Tests passed"

# 7. Verify build
go build ./cmd/search_engine
echo "✅ Build successful"

echo "🎉 All linting checks passed!"
```

### Linting Configuration

The project uses golangci-lint with the following enabled linters (configured in CI):

- **errcheck**: Check for unchecked errors
- **gosimple**: Suggest code simplifications
- **govet**: Go vet analysis
- **ineffassign**: Detect ineffectual assignments
- **staticcheck**: Advanced static analysis
- **typecheck**: Type checking
- **unused**: Find unused code
- **misspell**: Find commonly misspelled words
- **gofmt**: Check code formatting
- **goimports**: Check import formatting

### Code Quality Standards

- No unused variables or imports
- All public functions must have documentation
- Error handling must be explicit (no `_` discarding of errors)
- Use meaningful variable names (avoid single letters except for short loops)
- Prefer composition over inheritance
- Keep functions focused and small (generally under 50 lines)
- Struct literals must use keyed fields for clarity
- Avoid copying lock values (use pointers to structs containing mutexes)
- Handle all error returns explicitly

## Testing Procedures

### Testing Framework

- Use Go's built-in `testing` package
- Test files should end with `_test.go`
- Benchmark files should include `benchmark_test.go`

### Test Cleanup

- **Automatic cleanup**: Test directories are automatically tracked and cleaned up after each test run
- **Test directory pattern**: Tests create unique directories using `test_data_<timestamp>` format
- **Manual cleanup**: Use `./scripts/cleanup_test_data.sh` to manually remove any leftover test directories
- **Thread-safe tracking**: Test directory registration is mutex-protected for parallel test execution

### Test Organization

- **Unit Tests**: Test individual functions and methods
- **Integration Tests**: Test component interactions
- **API Tests**: Test HTTP endpoints using `httptest`

### Test Coverage Expectations

- Aim for **80%+ test coverage** for core functionality
- **100% coverage** required for critical paths (search algorithms, indexing)
- All public APIs must have comprehensive tests

### Test Naming Convention

```go
func TestFunctionName(t *testing.T)              // Basic test
func TestFunctionName_EdgeCase(t *testing.T)     // Edge case test
func TestFunctionName_ErrorCondition(t *testing.T) // Error condition test
func BenchmarkFunctionName(b *testing.B)         // Benchmark test
```

### Test Structure

Follow the **Arrange-Act-Assert** pattern:

```go
func TestSearchDocuments(t *testing.T) {
    // Arrange
    engine := NewTestEngine()
    documents := []Document{...}

    // Act
    results, err := engine.Search("query")

    // Assert
    assert.NoError(t, err)
    assert.Len(t, results, expectedCount)
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with detailed coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test
go test -run TestSearchDocuments ./internal/search

# Run benchmarks
go test -bench=. ./internal/typoutil
```

## Project Setup

### Prerequisites

- **Go 1.24.2 or later**
- **Git** for version control
- **curl** for API testing (optional)

### Development Environment Setup

1. **Clone the repository:**

```bash
git clone https://github.com/gcbaptista/go-search-engine.git
cd go-search-engine
```

2. **Install dependencies:**

```bash
go mod tidy
```

3. **Verify installation:**

```bash
go build ./cmd/search_engine
```

4. **Run the application:**

```bash
go run cmd/search_engine/main.go
```

5. **Verify the server is running:**

```bash
curl http://localhost:8080/health
```

### Development Dependencies

- **testing**: Built-in Go testing framework
- **httptest**: For API endpoint testing
- **gin**: Web framework for HTTP routing

### Configuration

- **Data Directory**: `./search_data` (configurable in `main.go`)
- **Default Port**: 8080
- **API Documentation**: Available in `api-spec.yaml`

### IDE Setup Recommendations

- **VS Code**: Install Go extension for syntax highlighting and debugging
- **GoLand**: Full-featured Go IDE with built-in tools
- **Vim/Neovim**: Use vim-go plugin for Go development

## Development Guidelines

### Commit Message Format

Use conventional commits format for clear change tracking:

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `test:` - Adding or updating tests
- `refactor:` - Code refactoring
- `perf:` - Performance improvements
- `style:` - Code style changes

**Example:**

```bash
git commit -m "feat: add fuzzy search functionality

- Implement Levenshtein distance algorithm
- Add configurable typo tolerance settings
- Include comprehensive test coverage
- Update API documentation"
```

### Documentation Standards

- **Public APIs**: Must have complete documentation
- **Examples**: Include usage examples for complex features
- **Field naming**: Follow the [Field Naming Guide](./FIELD_NAMING_GUIDE.md)
- **API changes**: Update OpenAPI specification in `api-spec.yaml`
- **Performance**: Document optimization decisions

### Performance Considerations

- **Benchmark new algorithms** especially in search and indexing
- **Profile memory usage** for large document sets
- **Consider typo tolerance performance** (see [Typo Optimization Guide](./TYPO_OPTIMIZATION_SUMMARY.md))
- **Test with realistic data sizes**

### Security Guidelines

- **Validate all input** from API requests
- **Sanitize file paths** in persistence layer
- **Limit resource usage** (memory, CPU) for large operations
- **Document security assumptions** in code comments

### Development Workflow

1. **Follow coding conventions** outlined in this document
2. **Write comprehensive tests** for new functionality
3. **Update documentation** including:
   - Code comments for public functions
   - README.md if adding new features
   - API documentation in `api-spec.yaml`
4. **Run complete linting suite** after major tasks (as per Post-Major Task Requirements)
5. **Verify build and tests** before considering work complete

---

## Additional Resources

- **[Complete Documentation](./README.md)** - All project documentation
- **[Performance Guide](./TYPO_OPTIMIZATION_SUMMARY.md)** - Optimization details
- **[Dashboard Guide](./DASHBOARD_GUIDE.md)** - Web interface documentation
- **[Field Naming Guide](./FIELD_NAMING_GUIDE.md)** - Critical naming conventions
- **[Progress Tracker](./PROGRESS.md)** - Development milestones
- **[API Specification](../api-spec.yaml)** - Complete OpenAPI documentation

For questions or support, please open an issue on the [GitHub repository](https://github.com/gcbaptista/go-search-engine).
