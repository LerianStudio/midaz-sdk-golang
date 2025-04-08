# Contributing to Midaz Go SDK

Thank you for your interest in contributing to the Midaz Go SDK! This document provides guidelines and instructions for contributing to this project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Process](#development-process)
  - [Setting Up Your Environment](#setting-up-your-environment)
  - [Making Changes](#making-changes)
  - [Running Tests](#running-tests)
  - [Generating Documentation](#generating-documentation)
- [Submitting Changes](#submitting-changes)
  - [Pull Requests](#pull-requests)
  - [Commit Messages](#commit-messages)
- [Coding Standards](#coding-standards)
  - [Go Guidelines](#go-guidelines)
  - [Documentation](#documentation)
  - [Testing](#testing)
- [Issue Reporting](#issue-reporting)

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](../CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to the project maintainers.

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork to your local machine
3. Set up the development environment (see [Setting Up Your Environment](#setting-up-your-environment))
4. Create a new branch for your changes
5. Make your changes and run tests
6. Push your changes to your fork
7. Submit a pull request

## Development Process

### Setting Up Your Environment

To set up your development environment:

1. Install Go (version 1.19 or higher)
2. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/midaz.git
   cd midaz/sdks/go-sdk
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```

### Making Changes

When making changes:

1. Create a new branch from the main branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```
2. Make your changes
3. Add tests for your changes
4. Update documentation as needed
5. Ensure all tests pass

### Running Tests

Run tests with:

```bash
make test
```

For faster test iterations during development:

```bash
make test-fast
```

To generate a test coverage report:

```bash
make coverage
```

### Generating Documentation

Generate documentation with:

```bash
make docs
```

To start a documentation server:

```bash
make godoc
```

## Submitting Changes

### Pull Requests

When submitting a pull request:

1. Ensure your code follows the project's coding standards
2. Include tests for your changes
3. Update documentation as needed
4. Reference any related issues in your pull request description
5. Describe your changes in detail
6. Request a review from maintainers

### Commit Messages

Follow the conventional commits format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Types:
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Changes that do not affect the meaning of the code
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `perf`: A code change that improves performance
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to the build process or auxiliary tools

Example:
```
feat(pagination): add cursor-based pagination support

Implement cursor-based pagination for all list operations
to improve performance with large datasets.

Closes #123
```

## Coding Standards

### Go Guidelines

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` to format your code
- Run `go vet` and `golint` on your code before submitting
- Package names should be lowercase, single-word names
- Exported functions and variables should be camel case with the first letter capitalized
- Unexported functions and variables should be camel case with the first letter lowercase
- Comments for exported symbols should start with the symbol name

### Documentation

- Document all exported functions, types, and variables
- Include examples where appropriate
- Keep documentation up-to-date with code changes
- Use proper grammar and clear language
- Documentation comments should follow the Go convention for godoc

### Testing

- Write unit tests for all new code
- Use table-driven tests where appropriate
- Aim for at least 80% test coverage
- Tests should be clear and easy to understand
- Use meaningful test names
- Avoid global state in tests

## Issue Reporting

When reporting issues:

1. Use the issue templates provided
2. Include steps to reproduce the issue
3. Include expected and actual results
4. Include any error messages or logs
5. Include the version of the SDK you are using
6. Include your environment details (Go version, OS, etc.)

Thank you for contributing to the Midaz Go SDK!