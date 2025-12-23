# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.2] - 2025-12-23

### Added
- New `pkg/pathutil` package for secure file path handling
- Secure file operation wrappers: `SecureOpen`, `SecureCreate`, `SecureReadFile`, `ReadValidated`

### Changed
- Renamed packages to follow Go naming conventions:
  - `examples/async_example` → `asyncexample`
  - `examples/grpc_example` → `grpcexample`
  - `examples/http_example` → `httpexample`
  - `pkg/broker/api` → `pkg/broker/brokerapi`
  - `pkg/validator/common` → `pkg/validator/validation`
  - `pkg/validator/http` → `pkg/validator/httpvalidator` (avoids stdlib conflict)
- Improved error handling: all type assertions and `json.Marshal` calls now check errors
- Added context support to PostgreSQL operations (`PingContext`, `ExecContext`)
- Used `net.ListenConfig` for context-aware network listeners
- Replaced `nil` request body with `http.NoBody` for clarity
- Pre-allocated slices where capacity is known

### Fixed
- All 48 golangci-lint issues resolved
- Directory permissions changed from 0755 to 0750 (G301)
- Path traversal protection via centralized `pathutil` package (G304)
- Import grouping with local-prefix for goimports

### Security
- Centralized file operations in `pkg/pathutil` with path validation
- All file open/create/read operations now go through secure wrappers
- Path traversal attacks prevented by validating paths before file operations

## [0.3.1] - 2025-12-22

### Changed
- Migrated golangci-lint configuration to v2 format
- Moved formatters (gofmt, goimports) to dedicated `formatters` section
- Updated linter exclusions to use new `linters.exclusions` structure

### Fixed
- GoReleaser Docker build using separate `Dockerfile.goreleaser` for pre-built binaries
- Homebrew formula upload skipped until token configured

## [0.3.0] - 2025-12-22

### Added
- Shared HTTP client abstraction for CLI commands (`cmd/contract/commands/http.go`)

### Changed
- Replace deprecated `exportloopref` linter with `copyloopvar` for Go 1.22+ compatibility
- Refactored CLI commands to reduce cyclomatic complexity:
  - `fetch.go`: 20 → ~9 (extracted `fetchContractList`, `fetchAndSaveContract`)
  - `candeploy.go`: 16 → ~6 (extracted `printCanDeployResult`)
  - `publish.go`: 16 → ~8 (extracted `ensureTagInContract`, `publishContract`)
- Refactored broker API to reduce cyclomatic complexity:
  - `deploy.go` `CanDeploy`: 17 → ~7 (extracted provider/consumer verification helpers)
  - `deploy.go` `GetMatrix`: 18 → ~8 (extracted `extractServices`, `groupContractsByPair`, `buildMatrixCell`)
- Refactored matching rules compiler to extract helper functions (`compileRegexRule`, `compileIncludeRule`, `compileNullOrRule`)
- Changed `contract.Interaction` to pointer type in validator interfaces for better performance
- Changed `ValidationError` to use pointer receivers consistently
- Improved struct field alignment across contract model types
- Pre-allocate slices where capacity is known for better memory efficiency

### Fixed
- All golangci-lint gocyclo violations resolved (threshold: 15)

## [0.2.0] - 2025-12-22

### Added
- CODE_OF_CONDUCT.md with development-focused guidelines
- .github/CODEOWNERS for automatic PR review assignments
- Go Reference badge in README
- Comprehensive README with full documentation
- CONTRIBUTING.md with development guidelines
- LICENSE (MIT)
- GitHub Actions CI pipeline (test, lint, build)
- golangci-lint configuration
- SECURITY.md with vulnerability reporting guidelines
- Dependabot configuration for automated dependency updates
- Issue and PR templates
- Makefile for build automation
- gRPC examples (consumer and provider tests with unary, server-stream, bidirectional)
- Async messaging examples (event-driven and saga sequence patterns)
- Architecture documentation (`docs/architecture.md`)
- Dockerfile with multi-stage build for broker
- docker-compose.yml with profiles for filesystem, PostgreSQL, and S3 backends
- GoReleaser configuration for automated releases
- GitHub Actions release workflow

### Fixed
- Bidirectional stream sequence synchronization in gRPC mock

### Testing
- Added comprehensive test suite for HTTP validator (39% coverage)
- Added comprehensive test suite for consumer DSL (50% coverage)
- Added comprehensive test suite for gRPC validator and mock server (70% coverage)
- Added comprehensive test suite for async validator and message capture (71% coverage)

## [0.1.0] - 2025-12-22

### Added
- Core contract model with JSON serialization (`770226b`)
- Matching rules engine with support for type, regex, include, integer, decimal, each_like, optional, and null_or matchers (`99a3c90`)
- HTTP validator and mock server (`3c97edf`)
- Broker service with filesystem storage backend (`b248c32`)
- CLI tool with commands: publish, fetch, verify, can-deploy, tag, broker (`07c6c44`)
- Consumer DSL for defining contract expectations (`2b3e88b`)
- Provider verification framework (`2b3e88b`)
- HTTP examples (consumer and provider tests) (`2b3e88b`)
- gRPC validator and mock server with streaming support (`e311b59`)
- Async message validator and capture mechanism (`5786f58`)
- S3 storage backend for cloud deployments (`fa0f081`)
- PostgreSQL storage backend for production use (`fa0f081`)
- Test utilities package with fixtures and mocks (`877418a`)
- Comprehensive test coverage for storage, matching, and contract packages (`877418a`, `7ef58bb`)
- Common header validation module shared across validators (`4e8a8a9`)

### Fixed
- HTTP example tests compatibility (`768fcc9`)
- Non-constant format string in gRPC mock (`db937a4`)
- Error handling improvements across CLI commands (`3071cce`)
- Body size limits for HTTP request/response validation (`3071cce`)

### Security
- SQL injection vulnerability in PostgreSQL storage backend (`190903b`)
- Path traversal vulnerability in filesystem storage backend (`190903b`)

### Changed
- Consolidated storage error types into single interface (`006225b`)
- Extracted common header validation to reduce code duplication (`4e8a8a9`)
