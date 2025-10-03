# Changelog

All notable changes to the Midaz Go SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

import "github.com/LerianStudio/midaz-sdk-golang/v2"

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.1.0-beta.5...v2.1.0-beta.6)
Contributors: Guilherme Moreira Rodrigues

### 🐛 Bug Fixes
- **Configuration Update**: Resolved a misconfiguration in the `github-actions-gptchangelog` action. This fix enhances the stability and reliability of our continuous integration and deployment workflows, ensuring they run smoothly without the need for manual intervention. Users will experience more consistent and dependable automated processes as a result.

### 🔧 Maintenance
- **Environment Configuration**: Improved the setup of automated workflows, contributing to the overall robustness and efficiency of our development and deployment pipeline.


## [v2.1.1-beta.1] - 2025-10-03

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.1.0...v2.1.1-beta.1)
Contributors: Arnaldo Pereira

### 🐛 Bug Fixes
- **Consistent API Versioning**: Resolved discrepancies in API versioning across services, ensuring more reliable and predictable interactions between components. Users will experience smoother service integration and fewer unexpected behaviors (#114).

### 🔧 Maintenance
- **Code Cleanup and Refactoring**: Improved code readability and maintainability by reducing the codebase by 27 lines. This behind-the-scenes enhancement supports better system management and future development, although it doesn't directly affect user-facing features.


## [v2.1.0-beta.4] - 2025-09-30

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.1.0-beta.3...v2.1.0-beta.4)
Contributors: Guilherme Moreira Rodrigues

### 🐛 Bug Fixes
- **Improved GitHub Actions Compatibility**: Updated the configuration to use the 'with' keyword for input parameters instead of 'env'. This change prevents potential execution errors, ensuring that the CI/CD pipeline runs smoothly and reliably, which is crucial for maintaining consistent deployment processes.

### 🔧 Maintenance
- **Configuration Update**: This behind-the-scenes improvement aligns our setup with the latest GitHub Actions best practices, reducing the risk of future compatibility issues and enhancing the overall stability of our development workflow.


## [v2.0.0] - 2025-08-04

This major release of the midaz-sdk-golang introduces significant enhancements to deployment processes and system architecture, alongside improvements in documentation and code quality.

### ⚠️ Breaking Changes
- **Backend/Config**: Models have transitioned to utilize Midaz entities, requiring updates to backend service integrations. This change enhances consistency and future-proofs the architecture. Users should review and adjust their model interfaces accordingly. [Migration Guide](#)

### ✨ Features  
- **Config**: A new release flow now supports Hotfixes (HF) and Breaking Changes (BC), offering more flexible and controlled deployment options for smoother updates and rollbacks.

### 🐛 Bug Fixes
- **Frontend**: Resolved various linting issues and improved variable naming conventions, enhancing code clarity and reducing potential errors.

### 📚 Documentation
- **Docs**: Expanded to include new accounting features and removed outdated scale fields, providing clearer guidance and reducing confusion for users.

### 🔧 Maintenance
- **Build/Deps**: Updated dependencies, including `github.com/LerianStudio/lib-commons` from 1.8.0 to 1.12.1, addressing security vulnerabilities and ensuring compatibility with the latest features.
- **Build/Docs/Frontend/Test**: Comprehensive cleanup of golangci-lint violations, improving code quality and maintainability across multiple components.

This release focuses on enhancing user experience through improved deployment processes, clearer documentation, and robust code quality standards.

This changelog is structured to provide users with a clear understanding of the changes, focusing on the impact and benefits of the new version. It includes essential details about breaking changes, new features, bug fixes, documentation updates, and maintenance improvements, all presented in a user-friendly format.

## [v2.0.0-beta.1] - 2025-08-04

This release introduces significant enhancements to the midaz-sdk-golang, including a major transition to Midaz entity models, improved code quality, and updated documentation. These changes aim to improve data consistency, maintainability, and user experience.

### ⚠️ Breaking Changes
- **Backend**: Transition to Midaz entities for all models. This change enhances data consistency and aligns with Midaz standards. **Action Required**: Update your integrations and data handling processes to accommodate these new entities. [Migration Guide](#)

### ✨ Features  
- **Backend**: Introduced Midaz entity models, offering a standardized and robust data structure that supports future scalability and integration with other Midaz services. This update is crucial for maintaining compatibility with future SDK updates.

### 🐛 Bug Fixes
- **Test**: Adjusted routing methods and removed obsolete scale fields, improving test accuracy and reliability, ensuring smoother testing processes.

### ⚡ Performance
- **Frontend**: Refactored code to replace 'interface{}' with 'any', improving code readability and maintainability, which enhances developer experience and aligns with modern Go practices.

### 🔄 Changes
- **Build/Test**: Cleaned up golangci-lint violations across multiple components, resulting in improved code quality and reduced technical debt.

### 📚 Documentation
- **Docs**: Updated documentation to include new accounting features and removed outdated scale fields, ensuring users have access to the latest feature information and guidelines.

### 🔧 Maintenance
- **Dependencies**: Upgraded dependency versions to ensure compatibility with the latest security patches and performance improvements.
- **Code Quality**: Various linting improvements, including variable renaming and code standardization, enhancing overall codebase maintainability.

This changelog provides a clear and concise overview of the changes in version 2.0.0, focusing on user impact and necessary actions. It highlights the benefits of new features, improvements, and maintenance updates, ensuring users understand the importance and implications of this release.

## [v1.4.0-beta.1] - 2025-07-31

This release introduces a streamlined configuration process for faster updates and enhances system performance through key dependency updates.

### ✨ Features
- **New Release Flow for Configuration**: We've implemented a new release flow to support Hot Fix (HF) and Bug Correction (BC) processes. This enhancement ensures quicker deployment of critical updates, improving system stability and performance for all users.

### ⚡ Performance
- **Dependency Update**: Upgraded the `github.com/LerianStudio/lib-commons` library from version 1.8.0 to 1.12.1. This update brings performance improvements and bug fixes that enhance the efficiency and reliability of components like authentication and build processes.

### 🔧 Maintenance
- **Build System Robustness**: The dependency update not only improves performance but also ensures compatibility with the latest security patches, maintaining the robustness and security of our build system.

By focusing on these enhancements and maintenance updates, users can expect a more streamlined and efficient experience, with improved system reliability and performance.

=======
>>>>>>> 510f9ec (chore(release): Update CHANGELOG)
## [v1.3.0] - 2025-06-02

### ✨ Features
- Improve release flow by fixing the goreleaser file, enhancing the overall release process.

### 🔧 Maintenance
- Bump `go.opentelemetry.io/otel` from version 1.35.0 to 1.36.0.
- Bump `go.opentelemetry.io/otel/metric` from version 1.35.0 to 1.36.0.
- Bump `go.opentelemetry.io/otel/trace` from version 1.35.0 to 1.36.0.

## [v1.3.0] - 2025-06-02

### ✨ Features
- Improve release flow by fixing the goreleaser file, enhancing the overall release process.

### 🔧 Maintenance
- Bump `go.opentelemetry.io/otel` from version 1.35.0 to 1.36.0.
- Bump `go.opentelemetry.io/otel/metric` from version 1.35.0 to 1.36.0.
- Bump `go.opentelemetry.io/otel/trace` from version 1.35.0 to 1.36.0.

## [v1.3.0-beta.2] - 2025-05-27

### 🔧 Maintenance
- Bump `go.opentelemetry.io/otel/trace` from version 1.35.0 to 1.36.0 to ensure compatibility with the latest features and improvements (#38).
- Update CHANGELOG to reflect recent changes and maintain accurate project documentation.

## [v1.3.0-beta.1] - 2025-05-05

### ✨ Features
- Update `goreleaser` configuration to improve release flow, enhancing the efficiency and reliability of the release process.

### 📚 Documentation
- Update CHANGELOG with recent changes to ensure it reflects the latest updates and improvements.

## [v1.2.0] - 2025-05-05

### 🔧 Maintenance
- Rename `pluginAuth` to `AccessManager` and update related documentation for clarity and consistency.

### 📚 Documentation
- Update CHANGELOG to reflect recent changes and improvements in the project.

## [v1.1.0] - 2025-05-03

### ✨ Features
- Rebuild release steps using custom modules to streamline the deployment process.
- Add gosec security checks to Makefile to enhance code security.

### 🐛 Bug Fixes
- Correct goreleaser step in the release process to ensure successful builds.

### 🔄 Changes
- Rename `pluginAuth` to `pluginAuth` and update related documentation for clarity.
- Adjust logging in `observability-demo.go` to prevent unused variable warnings.

### 🔧 Maintenance
- Configure checkout tags in CI workflow to improve version control accuracy.
- Set CodeQL analysis on default execution and add CodeQL analysis step to workflow for enhanced code quality checks.
- Configure additional workflow steps to optimize CI/CD processes.
- Remove unused `debugLog` function from `client.go` and replace unused client parameter with underscore in `main.go` for cleaner code.

### 📚 Documentation
- Update documentation to reflect changes in `pluginAuth`.


ianStudio/midaz-sdk-golang/compare/v1.0.7...v1.1.0-beta.1) (2025-04-09)

### Features

* **docs:** improve documentation on auxiliary packages ([9cd23e8](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/9cd23e8251bbcf9080d4f6bd73d8b6b79d7f665f))

## [1.0.7](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.6...v1.0.7) (2025-04-08)

### Bug Fixes

* **readme:** alignment ([bb62be1](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/bb62be17112245645e80747f7f24761af40ce62f))
* **readme:** alignment ([a4ce92c](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/a4ce92cca5efbf322e0f14d3fc03b49deb1a71b0))

## [1.0.6](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.5...v1.0.6) (2025-04-08)

### Bug Fixes

* **readme:** minor ([590a02e](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/590a02e9b584380949420501a6b2446ac7688cb5))

## [1.0.5](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.4...v1.0.5) (2025-04-08)

### Bug Fixes

* **readme:** banner image ([c362c6c](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/c362c6c32f1a929641025854066fa943fbd92c6b))

## [1.0.4](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.3...v1.0.4) (2025-04-08)

### Bug Fixes

* **readme:** fixing readme banner ([3a6d42a](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/3a6d42ab3aa86eda9f47a64863e7d9763610ca51))

## [1.0.3](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.2...v1.0.3) (2025-04-08)

## [1.0.2](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.1...v1.0.2) (2025-04-08)

### Bug Fixes

* **tests:** time tests to comply with pipeline machine time ([1912dd0](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/1912dd0b994bdb7d06e2522bf1451b1014865c05))
* **tests:** time tests to comply with pipeline machine time ([bb7806f](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/bb7806ff4e381c3c82bdaec47b60f19d50445cf7))

## [1.0.1](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.0...v1.0.1) (2025-04-08)

### Bug Fixes

* **pipeline:** artifacts version ([6bb53f2](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/6bb53f2891d45ea6dc15b8a4f79c9fdbe97807e5))

## 1.0.0 (2025-04-08)

### Features

* **sdk:** init repo ([709cb58](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/709cb5813927c4c505cd7d3da45cbf370cc67273))

# Changelog

All notable changes to the Midaz Go SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

import "github.com/LerianStudio/midaz-sdk-golang/v2"

## [Unreleased]

### Added
- Initial SDK setup with core functionality
- Entity models and client implementation
- Validation, error handling, and configuration utilities
- Concurrency utilities and pagination support
- Retry mechanisms and observability integration
- Comprehensive documentation and examples
