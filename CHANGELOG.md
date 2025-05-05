# Changelog

All notable changes to the Midaz Go SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

import "github.com/LerianStudio/midaz-sdk-golang"

## [v1.2.0] - 2025-05-05

### 🔧 Maintenance
- Rename `pluginAccessManager` to `AccessManager` and update related documentation for clarity and consistency.

### 📚 Documentation
- Update CHANGELOG to reflect recent changes and improvements in the project.

## [v1.1.0] - 2025-05-03

### ✨ Features
- Rebuild release steps using custom modules to streamline the deployment process.
- Add gosec security checks to Makefile to enhance code security.

### 🐛 Bug Fixes
- Correct goreleaser step in the release process to ensure successful builds.

### 🔄 Changes
- Rename `pluginAuth` to `pluginAccessManager` and update related documentation for clarity.
- Adjust logging in `observability-demo.go` to prevent unused variable warnings.

### 🔧 Maintenance
- Configure checkout tags in CI workflow to improve version control accuracy.
- Set CodeQL analysis on default execution and add CodeQL analysis step to workflow for enhanced code quality checks.
- Configure additional workflow steps to optimize CI/CD processes.
- Remove unused `debugLog` function from `client.go` and replace unused client parameter with underscore in `main.go` for cleaner code.

### 📚 Documentation
- Update documentation to reflect changes in `pluginAccessManager`.


ianStudio/midaz-sdk-golang/compare/v1.0.7...v1.1.0-beta.1) (2025-04-09)

### Features

* **docs:** improve documentation on auxiliary packages ([9cd23e8](https://github.com/LerianStudio/midaz-sdk-golang/commit/9cd23e8251bbcf9080d4f6bd73d8b6b79d7f665f))

## [1.0.7](https://github.com/LerianStudio/midaz-sdk-golang/compare/v1.0.6...v1.0.7) (2025-04-08)

### Bug Fixes

* **readme:** alignment ([bb62be1](https://github.com/LerianStudio/midaz-sdk-golang/commit/bb62be17112245645e80747f7f24761af40ce62f))
* **readme:** alignment ([a4ce92c](https://github.com/LerianStudio/midaz-sdk-golang/commit/a4ce92cca5efbf322e0f14d3fc03b49deb1a71b0))

## [1.0.6](https://github.com/LerianStudio/midaz-sdk-golang/compare/v1.0.5...v1.0.6) (2025-04-08)

### Bug Fixes

* **readme:** minor ([590a02e](https://github.com/LerianStudio/midaz-sdk-golang/commit/590a02e9b584380949420501a6b2446ac7688cb5))

## [1.0.5](https://github.com/LerianStudio/midaz-sdk-golang/compare/v1.0.4...v1.0.5) (2025-04-08)

### Bug Fixes

* **readme:** banner image ([c362c6c](https://github.com/LerianStudio/midaz-sdk-golang/commit/c362c6c32f1a929641025854066fa943fbd92c6b))

## [1.0.4](https://github.com/LerianStudio/midaz-sdk-golang/compare/v1.0.3...v1.0.4) (2025-04-08)

### Bug Fixes

* **readme:** fixing readme banner ([3a6d42a](https://github.com/LerianStudio/midaz-sdk-golang/commit/3a6d42ab3aa86eda9f47a64863e7d9763610ca51))

## [1.0.3](https://github.com/LerianStudio/midaz-sdk-golang/compare/v1.0.2...v1.0.3) (2025-04-08)

## [1.0.2](https://github.com/LerianStudio/midaz-sdk-golang/compare/v1.0.1...v1.0.2) (2025-04-08)

### Bug Fixes

* **tests:** time tests to comply with pipeline machine time ([1912dd0](https://github.com/LerianStudio/midaz-sdk-golang/commit/1912dd0b994bdb7d06e2522bf1451b1014865c05))
* **tests:** time tests to comply with pipeline machine time ([bb7806f](https://github.com/LerianStudio/midaz-sdk-golang/commit/bb7806ff4e381c3c82bdaec47b60f19d50445cf7))

## [1.0.1](https://github.com/LerianStudio/midaz-sdk-golang/compare/v1.0.0...v1.0.1) (2025-04-08)

### Bug Fixes

* **pipeline:** artifacts version ([6bb53f2](https://github.com/LerianStudio/midaz-sdk-golang/commit/6bb53f2891d45ea6dc15b8a4f79c9fdbe97807e5))

## 1.0.0 (2025-04-08)

### Features

* **sdk:** init repo ([709cb58](https://github.com/LerianStudio/midaz-sdk-golang/commit/709cb5813927c4c505cd7d3da45cbf370cc67273))

# Changelog

All notable changes to the Midaz Go SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

import "github.com/LerianStudio/midaz-sdk-golang"

## [Unreleased]

### Added
- Initial SDK setup with core functionality
- Entity models and client implementation
- Validation, error handling, and configuration utilities
- Concurrency utilities and pagination support
- Retry mechanisms and observability integration
- Comprehensive documentation and examples
