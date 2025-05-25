# Contributing to Go Search Engine

Thank you for your interest in contributing to the Go Search Engine! 🎉

## 🚀 Quick Start

1. **Fork** the repository
2. **Clone** your fork: `git clone https://github.com/your-username/go-search-engine.git`
3. **Create** a feature branch: `git checkout -b feature/your-feature-name`
4. **Make** your changes
5. **Test** your changes: `go test ./...`
6. **Commit** your changes: `git commit -m "feat: add your feature"`
7. **Push** to your fork: `git push origin feature/your-feature-name`
8. **Create** a Pull Request

## 📋 Development Guidelines

### Code Style

- Follow standard Go conventions (`gofmt`, `golint`)
- Write clear, self-documenting code
- Add comments that **clarify logic**, not implementation history
- Use meaningful variable and function names
- Keep functions focused and small

### Testing

- Write unit tests for new functionality
- Ensure all tests pass: `go test ./...`
- Add integration tests for complex features
- Include performance tests for performance-critical code
- Test edge cases and error conditions

### Documentation

- Update relevant documentation in `docs/`
- Update API spec (`api-spec.yaml`) for API changes
- Add code comments for complex logic
- Update README if needed

## 🏗️ Project Structure

```
go-search-engine/
├── api/                 # HTTP API handlers
├── cmd/search_engine/   # Main application entry point
├── config/             # Configuration structures
├── docs/               # Documentation

├── internal/           # Internal packages
│   ├── analytics/      # Analytics and metrics
│   ├── engine/         # Core search engine
│   ├── indexing/       # Document indexing
│   ├── jobs/           # Background job management
│   ├── search/         # Search functionality
│   └── typoutil/       # Typo tolerance utilities
├── model/              # Data models
├── services/           # Service interfaces
└── store/              # Data storage
```

## 🧪 Testing Your Changes

### Local Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./internal/typoutil/

# Build the application
go build ./cmd/search_engine/
```

### Manual Testing

```bash
# Start the server
./search_engine

# Test basic functionality
curl -X POST http://localhost:8080/indexes \
  -H "Content-Type: application/json" \
  -d '{"name": "test", "searchable_fields": ["title"]}'
```

## 🎯 Areas for Contribution

### High Priority

- 🐛 **Bug fixes** - Always welcome!
- ⚡ **Performance improvements** - Especially in search and indexing
- 📚 **Documentation** - Help others understand the codebase
- 🧪 **Test coverage** - Improve reliability

### Medium Priority

- ✨ **New features** - Discuss in issues first
- 🔧 **Code cleanup** - Refactoring and optimization
- 📊 **Analytics enhancements** - Better insights and metrics

### Ideas for New Contributors

- Fix typos in documentation
- Improve API documentation
- Improve error messages
- Add validation for edge cases
- Write integration tests

## 🔄 Pull Request Process

1. **Use the PR template** - Fill out all relevant sections
2. **Link to issues** - Reference related issues
3. **Keep PRs focused** - One feature/fix per PR
4. **Write good commit messages** - Use conventional commits
5. **Respond to feedback** - Address review comments promptly

### Commit Message Format

```
type(scope): description

feat(search): add fuzzy search support
fix(api): handle empty query strings
docs(readme): update installation instructions
perf(typo): optimize Levenshtein distance calculation
```

## 🚫 What Not to Do

- Don't add comments about refactoring history
- Don't submit PRs without tests
- Don't break existing functionality without justification
- Don't add unnecessary dependencies
- Don't ignore the PR template

## 🤝 Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Assume good intentions

## 🆘 Getting Help

- 📚 Check the [documentation](./docs/)
- 💬 Start a [discussion](https://github.com/gcbaptista/go-search-engine/discussions)
- 🐛 Open an [issue](https://github.com/gcbaptista/go-search-engine/issues/new/choose)

## 📄 License

By contributing, you agree that your contributions will be licensed under the same license as the project.

---

**Happy coding!** 🚀
