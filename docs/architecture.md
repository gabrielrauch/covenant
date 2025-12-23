# Covenant Architecture

This document describes the internal architecture of Covenant, a consumer-driven contract testing framework for Go.

## Overview

Covenant follows the **consumer-driven contract testing** pattern, where:
1. **Consumers** define their expectations of provider APIs
2. **Contracts** capture these expectations in a portable format
3. **Providers** verify they meet consumer expectations
4. A **Broker** enables contract sharing and deployment verification

```
                              CONTRACT FLOW

    ╭──────────────╮                              ╭──────────────╮
    │              │                              │              │
    │   Consumer   │                              │   Provider   │
    │    Tests     │                              │    Tests     │
    │              │                              │              │
    ╰──────┬───────╯                              ╰───────┬──────╯
           │                                              │
           │  creates                            verifies │
           │                                              │
           ▼                                              ▼
    ╭─────────────────────────────────────────────────────────────╮
    │                                                             │
    │                     CONTRACT (JSON)                         │
    │                                                             │
    │   • Consumer/Provider metadata                              │
    │   • Expected interactions                                   │
    │   • Matching rules                                          │
    │                                                             │
    ╰────────────────────────────┬────────────────────────────────╯
                                 │
                                 │  publish / fetch
                                 ▼
                    ╭────────────────────────╮
                    │                        │
                    │    BROKER SERVICE      │
                    │                        │
                    │  • Store contracts     │
                    │  • Track versions      │
                    │  • Verify deployments  │
                    │                        │
                    ╰────────────────────────╯
```

## Core Components

### 1. Contract Model (`pkg/contract`)

The contract model defines the structure for capturing consumer expectations:

```go
type Contract struct {
    Metadata      Metadata       // ID, version, consumer/provider info
    Interactions  []Interaction  // List of expected interactions
    MatchingRules MatchingRules  // Flexible matching rules
}
```

**Interactions** support three protocols:
- **HTTP**: RESTful API requests/responses
- **gRPC**: Unary and streaming RPC calls
- **Async**: Message queue/event-driven interactions

**Matching Rules** enable flexible validation:
- `type` - Match by type (string, number, boolean)
- `regex` - Match string patterns
- `integer` / `decimal` - Numeric validation
- `include` - Substring matching
- `each_like` - Array element matching
- `optional` - Allow missing fields
- `null_or` - Allow null or specific type

### 2. Consumer DSL (`pkg/consumer`)

Provides a fluent API for defining contracts:

```go
consumer.NewInteraction("get user by id").
    Given("user exists", map[string]any{"id": "123"}).
    WithHTTPRequest("GET", "/users/123").
    WillRespondWith(200).
    WithJSONBody(map[string]any{"id": "123", "name": "John"}).
    Build()
```

The DSL:
- Creates contract structures programmatically
- Manages mock servers for consumer testing
- Saves contracts to files for provider verification

### 3. Validators (`pkg/validator`)

Protocol-specific validation engines:

```
    ╭──────────────────────────────────────────────────╮
    │               pkg/validator                      │
    ├──────────────────────────────────────────────────┤
    │                                                  │
    │   ╭────────────╮  ╭──────────╮  ╭──────────╮     │
    │   │ validation │  │   http   │  │   grpc   │     │
    │   │            │  │          │  │          │     │
    │   │  Headers   │  │ Request  │  │ Unary    │     │
    │   │  Shared    │  │ Response │  │ Stream   │     │
    │   │  Utils     │  │ Mock     │  │ Mock     │     │
    │   ╰────────────╯  ╰──────────╯  ╰──────────╯     │
    │                                                  │
    │                 ╭──────────╮                     │
    │                 │  async   │                     │
    │                 │          │                     │
    │                 │ Messages │                     │
    │                 │ Sequence │                     │
    │                 │ Capture  │                     │
    │                 ╰──────────╯                     │
    │                                                  │
    ╰──────────────────────────────────────────────────╯
```

**HTTP Validator**:
- Validates requests match contract
- Mock server responds based on interactions
- Records requests for verification

**gRPC Validator**:
- Supports unary, server-stream, client-stream, bidirectional
- Mock server infrastructure
- Stream message validation

**Async Validator**:
- Message capture mechanism
- Sequence validation (for sagas)
- In-memory broker for testing

### 4. Provider Verification (`pkg/provider`)

Verifies provider implementations against contracts:

```go
verifier := provider.NewVerifier("orders-api", "1.0.0", serverURL)
verifier.OnProviderState("order exists", setupFunc)
verifier.AddContracts(contracts)
verifier.VerifyWithTesting(t)
```

Features:
- Provider state setup hooks
- HTTP request replay
- Result aggregation

### 5. Matching Engine (`pkg/matching`)

The core matching logic that compares expected vs actual values:

```
    ╭───────────────────────────────────────────────────────────╮
    │                      MATCHING ENGINE                      │
    ├───────────────────────────────────────────────────────────┤
    │                                                           │
    │   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐   │
    │   │  engine.go  │───▶│  rules.go   │───▶│  path.go    │   │
    │   │             │    │             │    │             │   │
    │   │ Compile     │    │ Type        │    │ JSONPath    │   │
    │   │ Execute     │    │ Regex       │    │ Resolution  │   │
    │   │ Report      │    │ Numeric     │    │ Navigation  │   │
    │   └─────────────┘    └─────────────┘    └─────────────┘   │
    │          │                                                │
    │          ▼                                                │
    │   ┌─────────────┐                                         │
    │   │structure.go │                                         │
    │   │             │                                         │
    │   │ Deep compare│                                         │
    │   │ Type check  │                                         │
    │   └─────────────┘                                         │
    │                                                           │
    ╰───────────────────────────────────────────────────────────╯
```

The engine:
1. Compiles matching rules into executable matchers
2. Resolves JSONPaths in data structures
3. Applies rules recursively
4. Reports mismatches with context

### 6. Broker Service (`pkg/broker`)

Central service for contract management:

```
    ╭───────────────────────────────────────────────────────────╮
    │                      BROKER SERVICE                       │
    ├───────────────────────────────────────────────────────────┤
    │                                                           │
    │   ╭─────────────────────────────────────────────────╮     │
    │   │                    API Layer                    │     │
    │   │                                                 │     │
    │   │  PUT /contracts         POST /verification      │     │
    │   │  GET /contracts/:id     GET /can-deploy         │     │
    │   │  PUT /tags              GET /matrix             │     │
    │   ╰─────────────────────────────────────────────────╯     │
    │                          │                                │
    │                          ▼                                │
    │   ╭─────────────────────────────────────────────────╮     │
    │   │               Storage Backends                  │     │
    │   │                                                 │     │
    │   │  ┌────────────┐ ┌────────────┐ ┌────────────┐   │     │
    │   │  │ Filesystem │ │     S3     │ │ PostgreSQL │   │     │
    │   │  │   (dev)    │ │  (cloud)   │ │   (prod)   │   │     │
    │   │  └────────────┘ └────────────┘ └────────────┘   │     │
    │   ╰─────────────────────────────────────────────────╯     │
    │                                                           │
    ╰───────────────────────────────────────────────────────────╯
```

**API Endpoints**:
- `PUT /contracts` - Publish contracts
- `GET /contracts/:consumer/:provider/:version` - Fetch contracts
- `POST /contracts/:consumer/:provider/verification` - Record verification
- `GET /can-deploy` - Check deployment safety
- `PUT /pacticipants/:name/versions/:version/tags/:tag` - Tag versions

### 7. CLI (`cmd/contract`)

Command-line interface for all operations:

```bash
covenant publish                # Publish contracts to broker
covenant fetch                  # Download contracts
covenant verify                 # Run provider verification
covenant can-deploy             # Check deployment compatibility
covenant tag                    # Tag participant versions
covenant broker                 # Start broker server
```

## Data Flow

### Consumer Test Flow

```
    ┌─────────────────────────────────────────────────────────┐
    │                  CONSUMER TEST FLOW                     │
    └─────────────────────────────────────────────────────────┘

         ╭───────────────╮
      1. │    Define     │
         │ Interactions  │
         ╰───────┬───────╯
                 │
                 ▼
         ╭───────────────╮
      2. │  Start Mock   │
         │    Server     │
         ╰───────┬───────╯
                 │
                 ▼
         ╭───────────────╮
      3. │ Consumer Code │──────────────╮
         │  Calls Mock   │              │
         ╰───────┬───────╯              │
                 │                      ▼
                 │              ╭───────────────╮
                 │              │    Validate   │
                 │              │    Request    │
                 │              ╰───────┬───────╯
                 │                      │
                 ▼                      ▼
         ╭───────────────╮      ╭───────────────╮
      4. │   Receive     │◀─────│    Return     │
         │   Response    │      │   Response    │
         ╰───────┬───────╯      ╰───────────────╯
                 │
                 ▼
         ╭───────────────╮
      5. │ Save Contract │
         │   to File     │
         ╰───────┬───────╯
                 │
                 ▼
         ╭───────────────╮
      6. │   Publish     │
         │  to Broker    │
         ╰───────────────╯
```

### Provider Verification Flow

```
    ┌─────────────────────────────────────────────────────────┐
    │               PROVIDER VERIFICATION FLOW                │
    └─────────────────────────────────────────────────────────┘

         ╭───────────────╮
      1. │    Fetch      │
         │  Contracts    │
         ╰───────┬───────╯
                 │
                 ▼
         ╭───────────────╮
      2. │  For Each     │◀──────────────╮
         │ Interaction   │               │
         ╰───────┬───────╯               │
                 │                       │
                 ▼                       │
         ╭───────────────╮               │
      3. │    Setup      │               │
         │Provider State │               │
         ╰───────┬───────╯               │
                 │                       │
                 ▼                       │
         ╭───────────────╮               │
      4. │    Replay     │               │
         │   Request     │               │
         ╰───────┬───────╯               │
                 │                       │
                 ▼                       │
         ╭───────────────╮               │
      5. │   Validate    │───────────────╯
         │   Response    │       more interactions?
         ╰───────┬───────╯
                 │
                 ▼
         ╭───────────────╮
      6. │    Record     │
         │   Results     │
         ╰───────────────╯
```

### Deployment Verification Flow

```
    ┌─────────────────────────────────────────────────────────┐
    │              DEPLOYMENT VERIFICATION FLOW               │
    └─────────────────────────────────────────────────────────┘

         ╭───────────────╮
         │  can-deploy   │
         │    query      │
         │               │
         │ • service     │
         │ • version     │
         │ • environment │
         ╰───────┬───────╯
                 │
                 ▼
         ╭───────────────────────────────────╮
         │         BROKER CHECKS             │
         │                                   │
         │  ┌─────────────────────────────┐  │
         │  │ All consumers verified?     │  │
         │  │           ✓                 │  │
         │  └─────────────────────────────┘  │
         │                                   │
         │  ┌─────────────────────────────┐  │
         │  │ All providers verified?     │  │
         │  │           ✓                 │  │
         │  └─────────────────────────────┘  │
         ╰───────────────┬───────────────────╯
                         │
                         ▼
          ┌───────────────────────────────┐
          │        DEPLOY DECISION        │
          ├───────────────────────────────┤
          │                               │
          │       ✓ SAFE TO DEPLOY        │
          │                               │
          │              or               │
          │                               │
          │       ✗ BLOCKED: reason       │
          │                               │
          └───────────────────────────────┘

```

## Package Dependencies

```
    ╭───────────────────────────────────────────────────────────────╮
    │                    PACKAGE DEPENDENCIES                       │
    ╰───────────────────────────────────────────────────────────────╯

    cmd/contract (CLI)
          │
          ├────────────▶ pkg/broker/client
          ├────────────▶ pkg/provider
          └────────────▶ pkg/contract

    pkg/broker/brokerapi
          │
          ├────────────▶ pkg/broker/storage
          └────────────▶ pkg/contract

    pkg/broker/storage
          │
          └────────────▶ pkg/pathutil

    pkg/consumer
          │
          ├────────────▶ pkg/contract
          └────────────▶ pkg/validator/http

    pkg/provider
          │
          ├────────────▶ pkg/contract
          └────────────▶ pkg/validator

    pkg/validator/*
          │
          ├────────────▶ pkg/contract
          └────────────▶ pkg/matching

    pkg/matching
          │
          └────────────▶ pkg/contract (types only)

    pkg/contract
          │
          └────────────▶ pkg/pathutil

    pkg/pathutil
          │
          └────────────▶ (no dependencies)
```

## Extension Points

### Adding a New Protocol

1. Define payload structure in `pkg/contract/model.go`
2. Create validator in `pkg/validator/<protocol>/`
3. Add DSL methods in `pkg/consumer/dsl.go`
4. Update provider verification if needed

### Adding a Storage Backend

1. Implement `storage.Storage` interface
2. Add factory function in `pkg/broker/storage/`
3. Register in broker server initialization

### Custom Matchers

1. Add matcher type in `pkg/contract/model.go`
2. Implement in `pkg/matching/rules.go`
3. Add DSL helper in `pkg/consumer/dsl.go`

## Design Decisions

### JSON for Contracts
Contracts are stored as JSON for:
- Human readability
- Language agnosticism
- Easy version control
- Simple serialization

### Protocol-Agnostic Core
The matching engine and contract model are protocol-agnostic, enabling:
- Consistent matching logic across protocols
- Easy addition of new protocols
- Shared test infrastructure

### Separation of Concerns
- **Consumer package**: Only knows how to define expectations
- **Provider package**: Only knows how to verify
- **Broker**: Only knows how to store and query
- **Validators**: Protocol-specific logic isolated

### No External Dependencies on Core
Core packages (`contract`, `matching`) avoid external dependencies to:
- Minimize version conflicts
- Enable embedding in various tools
- Keep the library lightweight
