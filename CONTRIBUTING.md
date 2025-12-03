# Contributing to Favicon Fetcher

Thank you for your interest in contributing to Favicon Fetcher! We welcome contributions from the community.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Style Guidelines](#style-guidelines)
- [Reporting Bugs](#reporting-bugs)
- [Feature Requests](#feature-requests)

## Code of Conduct

This project adheres to a code of conduct. By participating, you are expected to uphold this code. Please report unacceptable behavior to the project maintainers.

**Expected Behavior:**
- Be respectful and inclusive
- Accept constructive criticism
- Focus on what's best for the community
- Show empathy towards others

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/iprodev/Favicon-Fetcher.git
   cd favicon-fetcher
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/iprodev/Favicon-Fetcher.git
   ```

## Development Setup

### Prerequisites

- Go 1.22 or higher
- Git
- (Optional) libwebp for WebP support
- Make (optional, for convenience)

### Install Dependencies

```bash
go mod download
go mod tidy
```

### Build

```bash
# Standard build
go build -o favicon-server ./cmd/server

# With WebP support
go build -tags webp -o favicon-server ./cmd/server
```

### Run Locally

```bash
./favicon-server -log-level debug -cache-dir ./cache-dev
```

## Making Changes

### Branch Naming

Create a descriptive branch name:
- `feature/add-rate-limiting`
- `bugfix/fix-cache-key`
- `docs/update-readme`
- `refactor/cleanup-handler`

### Commit Messages

Follow the conventional commits specification:

```
type(scope): subject

body (optional)

footer (optional)
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(cache): add singleflight to prevent thundering herd

Implements singleflight pattern to deduplicate concurrent
requests for the same resource.

Closes #123
```

```
fix(security): prevent DNS rebinding attacks

Validates IP addresses immediately after DNS resolution
to prevent rebinding attacks.
```

## Testing

### Run Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./tests

# Specific test
go test -v ./tests -run TestFaviconHandler_ETag
```

### Write Tests

- Add unit tests for new features
- Ensure test coverage doesn't decrease
- Use table-driven tests when appropriate
- Mock external dependencies

**Example:**
```go
func TestNewFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "result", false},
        {"invalid input", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := NewFeature(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("unexpected error: %v", err)
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Benchmark Tests

```bash
go test -bench=. ./tests
```

## Submitting Changes

### Pull Request Process

1. **Update your fork**:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run tests**:
   ```bash
   go test ./...
   go vet ./...
   ```

3. **Commit your changes**:
   ```bash
   git add .
   git commit -m "feat: add new feature"
   ```

4. **Push to your fork**:
   ```bash
   git push origin feature/your-feature
   ```

5. **Create Pull Request**:
   - Go to GitHub
   - Click "New Pull Request"
   - Select your branch
   - Fill out the PR template

### Pull Request Guidelines

- **Title**: Clear and descriptive
- **Description**: Explain what and why
- **Tests**: Include tests for new features
- **Documentation**: Update docs if needed
- **Breaking Changes**: Clearly mark breaking changes
- **Link Issues**: Reference related issues

**PR Template:**
```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
Describe testing performed

## Checklist
- [ ] Tests pass locally
- [ ] Code follows style guidelines
- [ ] Documentation updated
- [ ] No new warnings
```

## Style Guidelines

### Go Code Style

Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

**Key Points:**
- Use `gofmt` for formatting
- Follow Go naming conventions
- Add comments for exported functions
- Keep functions small and focused
- Handle errors explicitly
- Use meaningful variable names

### Code Review

```bash
# Format code
gofmt -s -w .

# Vet code
go vet ./...

# Run linter (if available)
golangci-lint run
```

### Documentation

- Add GoDoc comments for exported types and functions
- Update README.md for user-facing changes
- Update API.md for API changes
- Include code examples where helpful

**Example:**
```go
// ResizeImage scales the given image to the specified size using
// bilinear interpolation. The aspect ratio is maintained by fitting
// the image within a square of the given size.
//
// Example:
//   img := image.NewRGBA(image.Rect(0, 0, 100, 100))
//   resized := ResizeImage(img, 32)
func ResizeImage(img image.Image, size int) image.Image {
    // ...
}
```

## Reporting Bugs

### Before Submitting

- Check existing issues
- Verify it's reproducible
- Check the FAQ

### Bug Report Template

```markdown
**Describe the bug**
Clear description of the bug

**To Reproduce**
Steps to reproduce:
1. Start server with '...'
2. Make request to '...'
3. See error

**Expected behavior**
What you expected to happen

**Actual behavior**
What actually happened

**Environment**
- OS: [e.g. Ubuntu 22.04]
- Go version: [e.g. 1.22]
- Version: [e.g. v1.0.0]

**Logs**
```
Paste relevant logs
```

**Additional context**
Any other information
```

## Feature Requests

We welcome feature requests! Please:

1. **Check existing issues** first
2. **Describe the problem** you're trying to solve
3. **Propose a solution** if you have one
4. **Consider the scope** - is it within the project's goals?

### Feature Request Template

```markdown
**Problem**
What problem does this solve?

**Proposed Solution**
How would you solve it?

**Alternatives**
Other solutions you've considered

**Additional Context**
Any other information
```

## Questions?

- **Documentation**: Check [docs/](docs/)
- **Discussions**: Use GitHub Discussions
- **Issues**: For bugs and features only

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to Favicon Fetcher! 🎉
