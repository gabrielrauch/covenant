# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
