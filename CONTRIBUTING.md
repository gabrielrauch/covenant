# Contributing to Covenant

Thank you for your interest in contributing to Covenant! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.24 or later
- Git

### Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/covenant.git
   cd covenant
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/gabrielrauch/covenant.git
   ```
4. Install dependencies:
   ```bash
   go mod download
   ```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Building

```bash
# Build the CLI
go build -o covenant ./cmd/contract

# Build and install
go install ./cmd/contract
```

## Code Style

### Formatting

All code must be formatted with `gofmt`:

```bash
gofmt -w .
```

### Linting

We use `golangci-lint` for linting:

```bash
golangci-lint run
```

### Guidelines

- Follow standard Go conventions and idioms
- Keep functions focused and small
- Write clear, self-documenting code
- Add comments for complex logic
- Ensure all exported types and functions have documentation

## Git Workflow

### Branch Naming

Use descriptive branch names:
- `feature/add-grpc-streaming` - New features
- `fix/validation-error` - Bug fixes
- `docs/update-readme` - Documentation changes
- `refactor/storage-interface` - Code refactoring

### Commit Messages

We use conventional commits with emoji prefixes:

| Emoji | Type | Description |
|-------|------|-------------|
| ✨ | `feat` | New feature |
| 🐛 | `fix` | Bug fix |
| 📚 | `docs` | Documentation |
| ♻️ | `refactor` | Code refactoring |
| ✅ | `test` | Adding tests |
| 🔒 | `security` | Security fix |
| ⚡ | `perf` | Performance improvement |
| 🔧 | `chore` | Maintenance tasks |

Example:
```
✨ feat: add gRPC streaming support

- Implement bidirectional streaming
- Add stream validation
- Update documentation
```

### Pull Request Process

1. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/your-feature
   ```

2. Make your changes and commit them following the commit message guidelines

3. Push to your fork:
   ```bash
   git push origin feature/your-feature
   ```

4. Open a Pull Request against `main`

5. Ensure all CI checks pass

6. Request review from maintainers

### PR Checklist

Before submitting a PR, ensure:

- [ ] Tests pass (`go test ./...`)
- [ ] Code is formatted (`gofmt -w .`)
- [ ] Linting passes (`golangci-lint run`)
- [ ] Documentation is updated if needed
- [ ] Commit messages follow the convention

## Project Structure

```
covenant/
├── cmd/contract/          # CLI tool entry point
│   ├── main.go            # Main CLI logic
│   └── commands/          # CLI command implementations
├── pkg/
│   ├── consumer/          # Consumer DSL and testing
│   ├── provider/          # Provider verification
│   ├── contract/          # Contract model and serialization
│   ├── broker/            # Contract broker server
│   │   ├── api/           # HTTP API handlers
│   │   └── storage/       # Storage backends (FS, Postgres, S3)
│   ├── validator/         # Protocol validators
│   │   ├── http/          # HTTP protocol validation
│   │   ├── grpc/          # gRPC protocol validation
│   │   ├── async/         # Async messaging validation
│   │   └── common/        # Shared validation utilities
│   ├── matching/          # Matching rules engine
│   └── testutil/          # Test utilities and fixtures
└── examples/              # Example usage
    ├── http/              # HTTP examples
    ├── grpc/              # gRPC examples
    └── async/             # Async examples
```

## Testing Guidelines

### Unit Tests

- Test files should be named `*_test.go`
- Use table-driven tests where appropriate
- Mock external dependencies
- Aim for high coverage on critical paths

Example:
```go
func TestValidateHeaders(t *testing.T) {
    tests := []struct {
        name     string
        expected map[string]string
        actual   map[string]string
        wantErr  bool
    }{
        {
            name:     "matching headers",
            expected: map[string]string{"Content-Type": "application/json"},
            actual:   map[string]string{"Content-Type": "application/json"},
            wantErr:  false,
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

### Integration Tests

- Place in the same package with `_test.go` suffix
- Use `t.TempDir()` for temporary files
- Clean up resources after tests

## Reporting Issues

### Bug Reports

When reporting bugs, please include:

1. Go version (`go version`)
2. Operating system
3. Steps to reproduce
4. Expected vs actual behavior
5. Error messages or logs

### Feature Requests

For feature requests, please describe:

1. The use case
2. Expected behavior
3. Any alternative solutions considered

## Getting Help

- Open an issue for questions
- Check existing issues for similar problems
- Review the documentation and examples

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
