# Contributing Guide

Thank you for your interest in contributing to Zencached! This guide outlines the process for contributing code, reporting issues, and improving documentation.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Commit Guidelines](#commit-guidelines)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Code Style](#code-style)
- [Documentation](#documentation)
- [Reporting Issues](#reporting-issues)

## Code of Conduct

We are committed to providing a welcoming and inclusive environment. Please treat all participants with respect.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally
3. Create a new branch for your work
4. Make your changes
5. Submit a pull request

## Development Setup

### Prerequisites

- Go 1.22 or later
- Git

### Clone and Setup

```bash
git clone https://github.com/rnojiri/zencached.git
cd zencached
go mod download
```

### Verify Setup

```bash
go test -v ./...
```

## Making Changes

### Create a Feature Branch

```bash
git checkout -b feature/my-feature
# or for bug fixes
git checkout -b fix/my-bugfix
```

Branch naming conventions:
- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation changes
- `refactor/description` - Code refactoring
- `test/description` - Test improvements

### Development Workflow

1. Make your changes in small, logical commits
2. Run tests frequently: `go test -v ./...`
3. Run linters: `go vet ./...` and `go fmt ./...`
4. Write or update tests as needed
5. Update documentation if needed

## Commit Guidelines

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, missing semicolons, etc.)
- `refactor`: Code refactoring without feature changes
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Build process or dependency changes

### Scope

The scope specifies what is affected (e.g., `client`, `telnet`, `compression`, `metrics`).

### Examples

```
feat(compression): add ZSTD compression support

- Implement DataCompressor interface for ZSTD
- Update configuration to support compression levels
- Add compression tests

Closes #123
```

```
fix(telnet): handle connection timeouts properly

- Increase default timeout from 1s to 5s
- Add exponential backoff for reconnections
- Add timeout error handling

Fixes #456
```

```
docs: improve API documentation

- Add more examples to README
- Document all configuration options
- Add troubleshooting section
```

## Testing

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run specific test
go test -v -run TestName ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Writing Tests

Tests should:
- Have clear, descriptive names
- Test both success and error cases
- Include benchmark tests for performance-critical code
- Use table-driven tests for multiple scenarios

### Example Test

```go
func TestZencachedSet(t *testing.T) {
    config := &Configuration{
        Nodes: []Node{{Host: "localhost", Port: 11211}},
    }
    client, err := New(config)
    if err != nil {
        t.Fatalf("Failed to create client: %v", err)
    }
    defer client.Shutdown()

    ctx := context.Background()
    result, err := client.Set(ctx, nil, []byte(""), []byte("key"), []byte("value"), 3600)
    if err != nil {
        t.Fatalf("Set failed: %v", err)
    }

    if result.Type != ResultTypeStored {
        t.Errorf("Expected ResultTypeStored, got %v", result.Type)
    }
}
```

### Running Benchmarks

```bash
go test -bench=. -benchmem ./...
```

### Example Benchmark

```go
func BenchmarkSet(b *testing.B) {
    config := &Configuration{
        Nodes: []Node{{Host: "localhost", Port: 11211}},
    }
    client, _ := New(config)
    defer client.Shutdown()

    ctx := context.Background()
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        client.Set(ctx, nil, []byte(""), []byte("key"), []byte("value"), 3600)
    }
}
```

## Pull Request Process

### Before Submitting

1. **Update your branch** from the latest main:
   ```bash
   git fetch origin
   git rebase origin/main
   ```

2. **Run tests locally**:
   ```bash
   go test -v ./...
   go vet ./...
   go fmt ./...
   ```

3. **Add/update tests** for your changes

4. **Update documentation** if needed

### PR Description Template

```markdown
## Description
Brief description of what this PR does.

## Changes
- Change 1
- Change 2
- Change 3

## Related Issue
Closes #<issue number>

## Testing
Describe how you tested these changes.

## Checklist
- [ ] I have tested the changes locally
- [ ] I have added/updated tests
- [ ] I have updated documentation
- [ ] All tests pass
- [ ] Code follows style guidelines
```

### PR Review Process

1. At least one approval from maintainers required
2. All CI checks must pass
3. No merge conflicts
4. Address review feedback

## Code Style

### Go Style Guidelines

We follow standard Go style conventions:

1. **Naming**:
   - CamelCase for exported identifiers
   - camelCase for unexported identifiers
   - Acronyms should be all caps: `HTTPResponse`, not `HttpResponse`

2. **Comments**:
   - Public functions should have comment starting with function name
   - Comments should be complete sentences
   - Package comment should start with package name

   ```go
   // Set stores a key-value pair in the cache.
   func (z *Zencached) Set(...) (*OperationResult, error) {
       // Implementation
   }
   ```

3. **Formatting**:
   ```bash
   go fmt ./...
   ```

4. **Linting**:
   ```bash
   go vet ./...
   ```

5. **Error Handling**:
   ```go
   // Good
   result, err := operation()
   if err != nil {
       return nil, fmt.Errorf("operation failed: %w", err)
   }

   // Not recommended
   result, _ := operation()
   ```

### Example Code

```go
package zencached

import (
    "context"
    "fmt"
)

// MyFeature represents a new feature.
type MyFeature struct {
    enabled bool
}

// NewMyFeature creates a new instance of MyFeature.
func NewMyFeature() *MyFeature {
    return &MyFeature{
        enabled: true,
    }
}

// DoSomething performs an operation.
func (m *MyFeature) DoSomething(ctx context.Context, key []byte) error {
    if !m.enabled {
        return fmt.Errorf("feature is disabled")
    }

    // Implementation
    return nil
}
```

## Documentation

### Documentation Changes

1. Update [README.md](../README.md) for user-facing changes
2. Update [API.md](./API.md) for API changes
3. Update [CONFIGURATION.md](./CONFIGURATION.md) for configuration changes
4. Update [EXAMPLES.md](./EXAMPLES.md) for new usage patterns
5. Update inline code comments

### Documentation Style

- Use clear, concise language
- Include code examples where relevant
- Update table of contents if adding sections
- Keep examples runnable and tested

## Reporting Issues

### Bug Reports

When reporting bugs, include:

1. **Description**: Clear description of the issue
2. **Steps to Reproduce**: Exact steps to reproduce
3. **Expected Behavior**: What should happen
4. **Actual Behavior**: What actually happens
5. **Environment**: Go version, OS, memcached version
6. **Error Message/Log**: Full error output
7. **Code Example**: Minimal code to reproduce

### Feature Requests

When requesting features:

1. **Description**: Clear description of desired feature
2. **Use Case**: Why is this feature needed?
3. **Alternative Solutions**: Other approaches considered
4. **Examples**: Usage examples

### Documentation Issues

When reporting documentation issues:

1. **Location**: Which file/section
2. **Issue**: What's unclear or wrong
3. **Suggestion**: How to improve

## Release Process

(For maintainers)

1. Update version in `go.mod`
2. Create release branch: `release/v1.2.3`
3. Update CHANGELOG
4. Merge to main
5. Tag release: `v1.2.3`
6. Publish release notes

## Questions?

- Open an issue for questions
- Check existing documentation first
- Search closed issues for similar topics

## Thank You!

We appreciate all contributions, whether code, documentation, or bug reports. Your help makes Zencached better!
