# Code Quality & Linting Guide

## Overview

This project uses Go 1.25.3, which is not yet fully supported by golangci-lint. Therefore, we rely on native Go tools for code quality checks.

## Quick Start

### Run All Checks (Recommended)

```bash
make check
```

This runs:
- `go vet` - detects suspicious code constructs
- `go build` - ensures all code compiles
- `gofmt` - checks code formatting

### Format Code

```bash
make fmt
```

Automatically formats all Go files according to the standard Go formatting rules.

### Check for Unused Code

```bash
make lint-unused
```

Uses `go vet` to find:
- Unused variables
- Unused imports
- Unused function parameters
- Dead code

### Full Static Analysis

```bash
make lint-all
```

Comprehensive check that includes:
1. `go vet` - suspicious constructs
2. `gofmt` - code formatting
3. `go build` - compilation check

## Available Commands

| Command | Description | Use When |
|---------|-------------|----------|
| `make check` | Quick checks with native tools | Before commit |
| `make fmt` | Auto-format code | Code not formatted |
| `make lint-unused` | Find unused code | Cleaning up |
| `make lint-all` | Full analysis | Before PR |
| `make lint` | golangci-lint (may fail) | When supported |

## Native Go Tools

### go vet

Examines Go source code and reports suspicious constructs:

```bash
go vet ./...
```

Detects:
- Unused variables
- Unreachable code
- Common mistakes
- Printf format errors
- Mutex issues

### go build

Compiles all packages:

```bash
go build ./...
```

Catches:
- Syntax errors
- Type errors
- Import issues
- Missing dependencies

### gofmt

Formats Go code:

```bash
# Check formatting
gofmt -l .

# Auto-fix formatting
go fmt ./...
```

## golangci-lint Configuration

Configuration file: `.golangci.yml`

**Note**: golangci-lint currently doesn't support Go 1.25+. The configuration is prepared for when support is added.

### When golangci-lint is Compatible

```bash
# Run all linters
make lint

# Auto-fix issues
make lint-fix
```

### Enabled Linters (in .golangci.yml)

**Core Linters:**
- `errcheck` - Check for unchecked errors
- `gosimple` - Simplify code
- `govet` - Reports suspicious constructs
- `ineffassign` - Detect ineffectual assignments
- `staticcheck` - Advanced static analysis
- `unused` - Check for unused constants, variables, functions
- `typecheck` - Type-check code

**Additional Linters:**
- `bodyclose` - Check HTTP response body closed
- `dupl` - Code clone detection
- `gocyclo` - Cyclomatic complexity
- `goconst` - Find repeated strings
- `gofmt` - Check code formatting
- `goimports` - Check import statements
- `gosec` - Security problems
- `misspell` - Spell checker
- `unconvert` - Remove unnecessary conversions
- `unparam` - Check for unused parameters
- `whitespace` - Detect leading/trailing whitespace
- `stylecheck` - Style checker

## CI/CD Integration

### GitHub Actions Example

```yaml
- name: Check code quality
  run: |
    make check
    make lint-unused
```

### Pre-commit Hook

Create `.git/hooks/pre-commit`:

```bash
#!/bin/bash
set -e

echo "Running code quality checks..."
make check

echo "✅ All checks passed!"
```

Make it executable:
```bash
chmod +x .git/hooks/pre-commit
```

## Best Practices

### Before Committing

```bash
make fmt        # Format code
make check      # Run checks
```

### Before Creating PR

```bash
make fmt
make lint-all
make test
```

### Periodic Cleanup

```bash
make lint-unused  # Find unused code
```

## Troubleshooting

### "go vet" Reports Issues

Fix the reported issues - they usually indicate real problems:
- Remove unused variables
- Fix Printf format strings
- Address concurrency issues

### "gofmt" Reports Formatting Issues

Run `make fmt` to auto-fix:
```bash
make fmt
```

### golangci-lint Version Error

This is expected with Go 1.25+. Use native tools instead:
```bash
make check      # Instead of make lint
make lint-all   # Full analysis
```

## Alternative Tools

### Install staticcheck

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
```

Run:
```bash
staticcheck ./...
```

### Install gosec (Security Scanner)

```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

Run:
```bash
gosec ./...
```

## Future: When golangci-lint Supports Go 1.25+

Once golangci-lint adds support for Go 1.25, you can use:

```bash
# Update golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run full linting suite
make lint

# Auto-fix issues
make lint-fix
```

## Summary

**Current Workflow (Go 1.25.3):**
1. Use `make check` for quick validation
2. Use `make fmt` to format code
3. Use `make lint-unused` to find unused code
4. Use `make lint-all` for comprehensive checks

**DO NOT:**
- Lower Go version to use golangci-lint
- Ignore `go vet` warnings
- Skip formatting checks

**When in Doubt:**
```bash
make check && make test
```

This ensures your code is properly formatted, compiles, and passes tests.

