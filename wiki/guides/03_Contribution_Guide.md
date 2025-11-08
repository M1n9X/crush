# Contribution Guide

## 🎯 Welcome Contributors!

Thank you for your interest in contributing to Crush! This guide will help you understand our development process, coding standards, and how to make meaningful contributions to the project.

## 🚀 Getting Started

### 1. Development Environment Setup

**Prerequisites**:
- Go 1.25.0 or later
- Git
- Make (optional, for using Makefile)
- Node.js 18+ (for documentation and tooling)

**Clone and Setup**:
```bash
# Fork the repository on GitHub first, then clone your fork
git clone https://github.com/YOUR_USERNAME/crush.git
cd crush

# Add upstream remote
git remote add upstream https://github.com/charmbracelet/crush.git

# Install dependencies
go mod download

# Install development tools
go install golang.org/x/tools/gopls@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install honnef.co/go/tools/cmd/staticcheck@latest

# Build the project
go build -o crush .

# Run tests
go test ./...
```

**Using Task Runner** (Recommended):
```bash
# Install Task
go install github.com/go-task/task/v3/cmd/task@latest

# View available tasks
task --list

# Run development setup
task setup

# Run tests
task test

# Run linting
task lint

# Build project
task build
```

### 2. Project Structure Understanding

```
crush/
├── cmd/                    # CLI commands and entry points
├── internal/              # Private application code
│   ├── app/              # Application orchestration
│   ├── config/           # Configuration management
│   ├── db/               # Database models and queries
│   ├── llm/              # AI/LLM integration
│   │   ├── agent/        # AI agent implementation
│   │   ├── provider/     # AI provider abstractions
│   │   └── tools/        # Tool system
│   ├── lsp/              # Language Server Protocol
│   ├── message/          # Message handling
│   ├── session/          # Session management
│   └── tui/              # Terminal UI
├── pkg/                   # Public packages (if any)
├── docs/                  # Documentation
├── scripts/               # Build and utility scripts
├── testdata/             # Test fixtures
├── .github/              # GitHub workflows and templates
├── Taskfile.yaml         # Task runner configuration
├── go.mod                # Go module definition
└── README.md             # Project overview
```

## 📝 Development Workflow

### 1. Issue-Based Development

**Before Starting**:
1. Check existing issues and discussions
2. Create or comment on relevant issues
3. Get consensus on approach for significant changes
4. Assign yourself to the issue

**Issue Types**:
- 🐛 **Bug**: Something isn't working correctly
- ✨ **Feature**: New functionality or enhancement
- 📚 **Documentation**: Documentation improvements
- 🔧 **Maintenance**: Code cleanup, refactoring, dependencies
- 🚀 **Performance**: Performance improvements

### 2. Branch Strategy

**Branch Naming Convention**:
```bash
# Feature branches
feature/issue-123-add-new-provider
feature/improve-context-management

# Bug fix branches
fix/issue-456-memory-leak
fix/lsp-connection-timeout

# Documentation branches
docs/update-installation-guide
docs/add-api-reference

# Maintenance branches
chore/update-dependencies
chore/refactor-message-service
```

**Workflow**:
```bash
# Start from main branch
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/issue-123-add-new-provider

# Make changes and commit
git add .
git commit -m "feat: add support for new AI provider

- Implement provider interface
- Add configuration schema
- Include unit tests
- Update documentation

Fixes #123"

# Push to your fork
git push origin feature/issue-123-add-new-provider

# Create pull request on GitHub
```

### 3. Commit Message Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/):

**Format**:
```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples**:
```bash
# Simple feature
git commit -m "feat: add OpenAI GPT-4 Turbo support"

# Bug fix with scope
git commit -m "fix(lsp): resolve connection timeout issues"

# Breaking change
git commit -m "feat!: redesign configuration schema

BREAKING CHANGE: Configuration format has changed.
See migration guide in docs/MIGRATION.md"

# With issue reference
git commit -m "fix: resolve memory leak in message streaming

Fixes #456"
```

## 🧪 Testing Standards

### 1. Test Structure

**Test Organization**:
```
internal/
├── service/
│   ├── service.go
│   ├── service_test.go          # Unit tests
│   └── service_integration_test.go  # Integration tests
└── testutil/                    # Test utilities
    ├── fixtures.go              # Test fixtures
    ├── mocks.go                 # Mock implementations
    └── helpers.go               # Test helpers
```

**Test Categories**:
- **Unit Tests**: Test individual functions/methods
- **Integration Tests**: Test component interactions
- **End-to-End Tests**: Test complete workflows

### 2. Writing Tests

**Unit Test Example**:
```go
func TestSessionService_Create(t *testing.T) {
    tests := []struct {
        name    string
        session Session
        want    error
    }{
        {
            name: "valid session",
            session: Session{
                Title: "Test Session",
            },
            want: nil,
        },
        {
            name: "empty title",
            session: Session{
                Title: "",
            },
            want: ErrEmptyTitle,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            service := setupTestService(t)
            
            err := service.Create(context.Background(), tt.session)
            
            if !errors.Is(err, tt.want) {
                t.Errorf("Create() error = %v, want %v", err, tt.want)
            }
        })
    }
}
```

**Integration Test Example**:
```go
func TestAgent_RunWithTools(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    // Setup test environment
    db := testutil.SetupTestDB(t)
    agent := setupTestAgent(t, db)
    
    // Test complete workflow
    events, err := agent.Run(context.Background(), "test-session", "List files in current directory")
    require.NoError(t, err)
    
    // Verify results
    var responses []AgentEvent
    for event := range events {
        responses = append(responses, event)
        if event.Type == AgentEventTypeComplete {
            break
        }
    }
    
    assert.NotEmpty(t, responses)
    assert.Contains(t, responses[len(responses)-1].Message.Content, "README.md")
}
```

**Test Utilities**:
```go
// testutil/fixtures.go
func SetupTestDB(t *testing.T) *sql.DB {
    t.Helper()
    
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    
    // Run migrations
    err = runMigrations(db)
    require.NoError(t, err)
    
    t.Cleanup(func() {
        db.Close()
    })
    
    return db
}

func NewTestSession(opts ...func(*Session)) Session {
    session := Session{
        ID:        uuid.New().String(),
        Title:     "Test Session",
        CreatedAt: time.Now().Unix(),
        UpdatedAt: time.Now().Unix(),
    }
    
    for _, opt := range opts {
        opt(&session)
    }
    
    return session
}
```

### 3. Running Tests

```bash
# Run all tests
task test

# Run tests with coverage
task test-coverage

# Run specific package tests
go test ./internal/session/...

# Run integration tests
go test -tags=integration ./...

# Run tests with race detection
go test -race ./...

# Run benchmarks
go test -bench=. ./...
```

## 🎨 Code Style & Standards

### 1. Go Style Guidelines

**Follow Standard Go Conventions**:
- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

**Naming Conventions**:
```go
// Packages: lowercase, single word
package session

// Types: PascalCase
type MessageService struct{}

// Functions/Methods: PascalCase for exported, camelCase for unexported
func (s *MessageService) CreateMessage() {}
func (s *MessageService) validateInput() {}

// Variables: camelCase
var sessionTimeout time.Duration
var maxRetries int

// Constants: PascalCase or SCREAMING_SNAKE_CASE for package-level
const DefaultTimeout = 30 * time.Second
const MAX_RETRY_ATTEMPTS = 3
```

**Error Handling**:
```go
// Define package-level errors
var (
    ErrSessionNotFound = errors.New("session not found")
    ErrInvalidInput    = errors.New("invalid input")
)

// Wrap errors with context
func (s *service) GetSession(id string) (*Session, error) {
    session, err := s.repo.GetByID(id)
    if err != nil {
        return nil, fmt.Errorf("failed to get session %s: %w", id, err)
    }
    return session, nil
}

// Use errors.Is and errors.As for error checking
if errors.Is(err, ErrSessionNotFound) {
    // handle not found
}
```

### 2. Code Organization

**Package Structure**:
```go
// Package declaration and imports
package session

import (
    "context"
    "fmt"
    
    "github.com/charmbracelet/crush/internal/db"
)

// Package-level constants and variables
const DefaultSessionTimeout = 30 * time.Minute

var ErrSessionExpired = errors.New("session expired")

// Types (interfaces first, then structs)
type Service interface {
    Create(ctx context.Context, session Session) error
    Get(ctx context.Context, id string) (*Session, error)
}

type service struct {
    queries *db.Queries
    timeout time.Duration
}

// Constructor functions
func NewService(queries *db.Queries) Service {
    return &service{
        queries: queries,
        timeout: DefaultSessionTimeout,
    }
}

// Methods (public first, then private)
func (s *service) Create(ctx context.Context, session Session) error {
    // implementation
}

func (s *service) validateSession(session Session) error {
    // implementation
}
```

### 3. Documentation Standards

**Package Documentation**:
```go
// Package session provides session management functionality for Crush.
//
// Sessions represent individual conversations with AI agents and maintain
// context, history, and metadata throughout the interaction lifecycle.
package session
```

**Function Documentation**:
```go
// CreateSession creates a new session with the given title and returns
// the created session with generated ID and timestamps.
//
// The session title must not be empty. If successful, the session is
// persisted to the database and can be retrieved using GetSession.
//
// Returns ErrEmptyTitle if the title is empty.
func (s *service) CreateSession(ctx context.Context, title string) (*Session, error) {
    // implementation
}
```

**Complex Logic Documentation**:
```go
func (a *agent) optimizeContext(messages []Message) []Message {
    // Context optimization algorithm:
    // 1. Calculate total token count for all messages
    // 2. If within limits, return as-is
    // 3. Otherwise, keep system message and recent messages that fit
    // 4. Summarize older messages if summarization is enabled
    
    totalTokens := a.calculateTokens(messages)
    if totalTokens <= a.maxContextTokens {
        return messages
    }
    
    // ... implementation
}
```

## 🔍 Code Review Process

### 1. Pull Request Guidelines

**PR Title Format**:
```
<type>[scope]: <description>

Examples:
feat: add support for Claude 3.5 Sonnet
fix(lsp): resolve connection timeout issues
docs: update installation guide for Windows
```

**PR Description Template**:
```markdown
## Description
Brief description of changes and motivation.

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing completed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests added/updated
- [ ] No breaking changes (or properly documented)
```

### 2. Review Criteria

**Code Quality**:
- [ ] Code is readable and well-documented
- [ ] Functions are focused and single-purpose
- [ ] Error handling is appropriate
- [ ] No obvious performance issues
- [ ] Security considerations addressed

**Testing**:
- [ ] Adequate test coverage
- [ ] Tests are meaningful and not just for coverage
- [ ] Edge cases considered
- [ ] Integration points tested

**Design**:
- [ ] Changes fit well with existing architecture
- [ ] Interfaces are well-designed
- [ ] No unnecessary complexity
- [ ] Follows established patterns

### 3. Review Process

1. **Automated Checks**: CI/CD pipeline runs tests and linting
2. **Peer Review**: At least one maintainer reviews the PR
3. **Discussion**: Address feedback and questions
4. **Approval**: PR approved by maintainer
5. **Merge**: Squash and merge to main branch

## 🚀 Release Process

### 1. Versioning

We follow [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### 2. Release Workflow

**Preparation**:
```bash
# Update version
git checkout main
git pull upstream main

# Update CHANGELOG.md
# Update version in relevant files

# Create release commit
git commit -m "chore: prepare release v1.2.0"

# Create and push tag
git tag -a v1.2.0 -m "Release v1.2.0"
git push upstream main --tags
```

**Automated Release**:
- GitHub Actions automatically builds and publishes releases
- Binaries are created for multiple platforms
- Package managers are updated

## 📚 Documentation Contributions

### 1. Documentation Types

- **API Documentation**: Generated from code comments
- **User Guides**: How-to guides and tutorials
- **Developer Documentation**: Architecture and contribution guides
- **Examples**: Code examples and use cases

### 2. Writing Guidelines

**Style**:
- Clear, concise language
- Active voice preferred
- Step-by-step instructions
- Code examples with explanations

**Structure**:
- Start with overview/introduction
- Provide prerequisites
- Include complete examples
- Add troubleshooting section

## 🤝 Community Guidelines

### 1. Code of Conduct

We follow the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/). Please be respectful, inclusive, and constructive in all interactions.

### 2. Communication Channels

- **GitHub Issues**: Bug reports, feature requests
- **GitHub Discussions**: Questions, ideas, general discussion
- **Discord**: Real-time chat and community support
- **Email**: Security issues and private matters

### 3. Getting Help

**Before Asking**:
1. Check existing documentation
2. Search issues and discussions
3. Review FAQ and troubleshooting guides

**When Asking**:
- Provide clear problem description
- Include relevant code/configuration
- Specify environment details
- Show what you've already tried

## 🎯 Contribution Ideas

### Good First Issues
- Documentation improvements
- Test coverage improvements
- Small bug fixes
- Code cleanup and refactoring

### Advanced Contributions
- New AI provider integrations
- Performance optimizations
- New tool implementations
- Architecture improvements

### Areas Needing Help
- Windows compatibility improvements
- Mobile/tablet terminal support
- Accessibility features
- Internationalization

---

*Thank you for contributing to Crush! Your efforts help make AI-assisted development accessible to everyone.*
