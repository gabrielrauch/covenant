# Covenant

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/gabrielrauch/covenant.svg)](https://pkg.go.dev/github.com/gabrielrauch/covenant)

**Consumer-driven contract testing for Go** — supporting HTTP, gRPC, and async messaging protocols.

Covenant enables teams to verify that services can communicate correctly without requiring integration tests. Consumers define contracts describing their expectations, and providers verify they meet those contracts.

## Features

- **Multi-Protocol Support**: HTTP REST, gRPC, and async messaging (Kafka, RabbitMQ, etc.)
- **Fluent DSL**: Intuitive Go API for defining consumer expectations
- **Flexible Matching**: Type matching, regex, includes, and more
- **Contract Broker**: Central repository for contract storage and verification results
- **CI/CD Integration**: `can-deploy` checks for safe deployments
- **Provider States**: Setup preconditions for provider verification
- **Multiple Storage Backends**: Filesystem, PostgreSQL, and S3

## Installation

```bash
go get github.com/gabrielrauch/covenant
```

For the CLI tool:

```bash
go install github.com/gabrielrauch/covenant/cmd/contract@latest
```

## Quick Start

### 1. Consumer Test

Define what your consumer expects from the provider:

```go
package myservice_test

import (
    "testing"
    "net/http"

    "github.com/gabrielrauch/covenant/pkg/consumer"
)

func TestOrderAPI_Consumer(t *testing.T) {
    // Define the expected interaction
    interaction := consumer.NewInteraction("get order by id").
        WithID("get-order").
        Given("order 123 exists", map[string]any{"id": "123"}).
        WithHTTPRequest("GET", "/orders/123").
        WillRespondWith(200).
        WithHeader("Content-Type", "application/json").
        WithJSONBody(map[string]any{
            "id":     "123",
            "status": "pending",
            "total":  99.99,
        }).
        Build()

    // Run test with automatic mock server
    consumer.RunTest(t, "checkout-ui", "orders-api",
        []*consumer.Interaction{interaction},
        func(mockURL string) {
            // Test your actual client code against the mock
            resp, err := http.Get(mockURL + "/orders/123")
            if err != nil {
                t.Fatal(err)
            }
            defer resp.Body.Close()

            if resp.StatusCode != 200 {
                t.Errorf("expected 200, got %d", resp.StatusCode)
            }
        },
        consumer.WithContractDir("./contracts"),
    )
}
```

### 2. Provider Verification

Verify your provider meets all consumer contracts:

```go
package myservice_test

import (
    "testing"
    "net/http/httptest"

    "github.com/gabrielrauch/covenant/pkg/provider"
)

func TestOrdersAPI_Provider(t *testing.T) {
    // Start your actual server
    server := httptest.NewServer(myOrdersHandler())
    defer server.Close()

    // Verify against contracts
    verifier := provider.NewVerifier(server.URL)

    // Setup provider states
    verifier.SetStateHandler("order 123 exists", func(params map[string]any) error {
        // Setup test data in your database/service
        return createTestOrder(params["id"].(string))
    })

    // Load and verify contracts
    result := verifier.VerifyFromDirectory("./contracts")

    if !result.Success {
        for _, err := range result.Errors {
            t.Errorf("Contract violation: %s - %s", err.Path, err.Message)
        }
    }
}
```

### 3. Matching Rules

Use flexible matchers for dynamic values:

```go
interaction := consumer.NewInteraction("create order").
    WithHTTPRequest("POST", "/orders").
    WillRespondWith(201).
    WithJSONBody(map[string]any{
        "id":         "generated-id",
        "created_at": "2024-01-01T00:00:00Z",
    }).
    Build()

// Match by type instead of exact value
interaction.WithMatchingRule("$.response.body.id", consumer.TypeMatcher())
interaction.WithMatchingRule("$.response.body.created_at", consumer.RegexMatcher(`^\d{4}-\d{2}-\d{2}T`))
```

Available matchers:
- `TypeMatcher()` - Match by type (string, number, boolean, etc.)
- `RegexMatcher(pattern)` - Match against regex pattern
- `IncludeMatcher(substring)` - String contains substring
- `IntegerMatcher()` - Must be an integer
- `DecimalMatcher()` - Must be a decimal number
- `EachLikeMatcher(min)` - Array with minimum elements

## CLI Commands

```bash
# Start the contract broker
covenant broker --port 8080 --storage filesystem --storage-path ./data

# Publish contracts to broker
covenant publish --broker http://localhost:8080 --contracts ./contracts

# Fetch contracts for a provider
covenant fetch --broker http://localhost:8080 --provider orders-api --output ./contracts

# Verify provider against contracts
covenant verify --broker http://localhost:8080 --provider orders-api --provider-url http://localhost:3000

# Check if safe to deploy
covenant can-deploy --broker http://localhost:8080 --service orders-api --version 1.2.0

# Tag contracts for environments
covenant tag --broker http://localhost:8080 --service orders-api --version 1.0.0 --tag production
```

## Contract Broker

The broker provides a central repository for contracts:

```bash
# Filesystem storage (development)
covenant broker --storage filesystem --storage-path ./data

# PostgreSQL storage (production)
covenant broker --storage postgres --postgres-url "postgres://user:pass@localhost/contracts"

# S3 storage (cloud)
covenant broker --storage s3 --s3-bucket my-contracts --s3-region us-east-1
```

### Broker API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/contracts` | POST | Publish a contract |
| `/contracts/{consumer}/{provider}/{version}` | GET | Fetch specific contract |
| `/contracts/{provider}` | GET | List contracts for provider |
| `/verifications` | POST | Record verification result |
| `/can-deploy/{service}/{version}` | GET | Check deployment safety |

## Project Structure

```
covenant/
├── cmd/contract/          # CLI tool
├── pkg/
│   ├── consumer/          # Consumer DSL and mock server
│   ├── provider/          # Provider verification
│   ├── contract/          # Contract model and serialization
│   ├── broker/            # Broker server and API
│   │   ├── api/           # HTTP handlers
│   │   └── storage/       # Storage backends
│   ├── validator/         # Protocol validators
│   │   ├── http/          # HTTP validation
│   │   ├── grpc/          # gRPC validation
│   │   └── async/         # Async messaging validation
│   └── matching/          # Matching rules engine
└── examples/              # Example usage
    ├── http/              # HTTP examples
    ├── grpc/              # gRPC examples
    └── async/             # Async examples
```

## Contract Format

Contracts are stored as JSON:

```json
{
  "metadata": {
    "id": "abc123",
    "version": "1.0.0",
    "consumer": { "name": "checkout-ui" },
    "provider": { "name": "orders-api" },
    "status": "published"
  },
  "interactions": [
    {
      "id": "get-order",
      "description": "get order by id",
      "protocol": "http",
      "provider_states": [
        { "name": "order 123 exists", "params": { "id": "123" } }
      ],
      "payload": {
        "http": {
          "request": { "method": "GET", "path": "/orders/123" },
          "response": {
            "status": 200,
            "body": { "id": "123", "status": "pending" }
          }
        }
      },
      "matching_rules": {
        "$.response.body.id": { "match": "type" }
      }
    }
  ]
}
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.
