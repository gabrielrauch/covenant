package consumer

import (
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

func TestNewContract(t *testing.T) {
	c := NewContract("consumer-app", "provider-api")
	if c == nil {
		t.Fatal("NewContract returned nil")
	}

	built := c.Build()
	if built.Metadata.Consumer.Name != "consumer-app" {
		t.Errorf("Consumer name = %s, want consumer-app", built.Metadata.Consumer.Name)
	}
	if built.Metadata.Provider.Name != "provider-api" {
		t.Errorf("Provider name = %s, want provider-api", built.Metadata.Provider.Name)
	}
	if built.Metadata.Status != contract.StatusDraft {
		t.Errorf("Status = %s, want draft", built.Metadata.Status)
	}
}

func TestContract_WithVersion(t *testing.T) {
	c := NewContract("consumer", "provider").
		WithVersion("2.0.0").
		Build()

	if c.Metadata.Version != "2.0.0" {
		t.Errorf("Version = %s, want 2.0.0", c.Metadata.Version)
	}
}

func TestContract_WithConsumerVersion(t *testing.T) {
	c := NewContract("consumer", "provider").
		WithConsumerVersion("1.2.3").
		Build()

	if c.Metadata.Consumer.Version != "1.2.3" {
		t.Errorf("Consumer version = %s, want 1.2.3", c.Metadata.Consumer.Version)
	}
}

func TestContract_WithProviderVersion(t *testing.T) {
	c := NewContract("consumer", "provider").
		WithProviderVersion("4.5.6").
		Build()

	if c.Metadata.Provider.Version != "4.5.6" {
		t.Errorf("Provider version = %s, want 4.5.6", c.Metadata.Provider.Version)
	}
}

func TestContract_WithTags(t *testing.T) {
	c := NewContract("consumer", "provider").
		WithTags("prod", "v1").
		Build()

	if len(c.Metadata.Tags) != 2 {
		t.Fatalf("Tags count = %d, want 2", len(c.Metadata.Tags))
	}
	if c.Metadata.Tags[0] != "prod" || c.Metadata.Tags[1] != "v1" {
		t.Errorf("Tags = %v, want [prod, v1]", c.Metadata.Tags)
	}
}

func TestContract_WithMatchingRule(t *testing.T) {
	c := NewContract("consumer", "provider").
		WithMatchingRule("$.body.id", contract.MatchingRule{Match: contract.MatchType_}).
		Build()

	if len(c.MatchingRules) != 1 {
		t.Fatalf("MatchingRules count = %d, want 1", len(c.MatchingRules))
	}
	rule, ok := c.MatchingRules["$.body.id"]
	if !ok {
		t.Fatal("Expected matching rule for $.body.id")
	}
	if rule.Match != contract.MatchType_ {
		t.Errorf("Rule match = %s, want type", rule.Match)
	}
}

func TestNewInteraction(t *testing.T) {
	i := NewInteraction("test interaction")
	if i == nil {
		t.Fatal("NewInteraction returned nil")
	}

	built := i.Build()
	if built.Description != "test interaction" {
		t.Errorf("Description = %s, want test interaction", built.Description)
	}
	if built.ID == "" {
		t.Error("ID should be auto-generated")
	}
}

func TestInteraction_WithID(t *testing.T) {
	i := NewInteraction("test").WithID("custom-id").Build()
	if i.ID != "custom-id" {
		t.Errorf("ID = %s, want custom-id", i.ID)
	}
}

func TestInteraction_Given(t *testing.T) {
	i := NewInteraction("test").
		Given("user exists", map[string]any{"id": "123"}).
		Build()

	if len(i.ProviderStates) != 1 {
		t.Fatalf("ProviderStates count = %d, want 1", len(i.ProviderStates))
	}
	state := i.ProviderStates[0]
	if state.Name != "user exists" {
		t.Errorf("State name = %s, want user exists", state.Name)
	}
	if state.Params["id"] != "123" {
		t.Errorf("State param id = %v, want 123", state.Params["id"])
	}
}

func TestInteraction_WithHTTPRequest(t *testing.T) {
	i := NewInteraction("get user").
		WithHTTPRequest("GET", "/users/123").
		WillRespondWith(200).
		Build()

	built := i.Build()
	if built.Protocol != contract.ProtocolHTTP {
		t.Errorf("Protocol = %s, want http", built.Protocol)
	}
	if built.Payload.HTTP == nil {
		t.Fatal("HTTP payload is nil")
	}
	if built.Payload.HTTP.Request.Method != "GET" {
		t.Errorf("Method = %s, want GET", built.Payload.HTTP.Request.Method)
	}
	if built.Payload.HTTP.Request.Path != "/users/123" {
		t.Errorf("Path = %s, want /users/123", built.Payload.HTTP.Request.Path)
	}
	if built.Payload.HTTP.Response.Status != 200 {
		t.Errorf("Status = %d, want 200", built.Payload.HTTP.Response.Status)
	}
}

func TestHTTPRequestBuilder_WithQuery(t *testing.T) {
	i := NewInteraction("search").
		WithHTTPRequest("GET", "/search").
		WithQuery(map[string]string{"q": "test"}).
		WillRespondWith(200).
		Build()

	built := i.Build()
	if built.Payload.HTTP.Request.Query["q"] != "test" {
		t.Errorf("Query q = %s, want test", built.Payload.HTTP.Request.Query["q"])
	}
}

func TestHTTPRequestBuilder_WithHeader(t *testing.T) {
	i := NewInteraction("auth").
		WithHTTPRequest("GET", "/protected").
		WithHeader("Authorization", "Bearer token").
		WillRespondWith(200).
		Build()

	built := i.Build()
	if built.Payload.HTTP.Request.Headers["Authorization"] != "Bearer token" {
		t.Errorf("Header Authorization = %s, want Bearer token",
			built.Payload.HTTP.Request.Headers["Authorization"])
	}
}

func TestHTTPRequestBuilder_WithJSONBody(t *testing.T) {
	i := NewInteraction("create").
		WithHTTPRequest("POST", "/users").
		WithJSONBody(map[string]any{"name": "John"}).
		WillRespondWith(201).
		Build()

	built := i.Build()
	body, ok := built.Payload.HTTP.Request.Body.(map[string]any)
	if !ok {
		t.Fatal("Body is not a map")
	}
	if body["name"] != "John" {
		t.Errorf("Body name = %v, want John", body["name"])
	}
	if built.Payload.HTTP.Request.Headers["Content-Type"] != "application/json" {
		t.Error("Content-Type header should be set for JSON body")
	}
}

func TestHTTPResponseBuilder_WithHeader(t *testing.T) {
	i := NewInteraction("get").
		WithHTTPRequest("GET", "/data").
		WillRespondWith(200).
		WithHeader("X-Custom", "value").
		Build()

	built := i.Build()
	if built.Payload.HTTP.Response.Headers["X-Custom"] != "value" {
		t.Errorf("Response header X-Custom = %s, want value",
			built.Payload.HTTP.Response.Headers["X-Custom"])
	}
}

func TestHTTPResponseBuilder_WithJSONBody(t *testing.T) {
	i := NewInteraction("get").
		WithHTTPRequest("GET", "/data").
		WillRespondWith(200).
		WithJSONBody(map[string]any{"result": "ok"}).
		Build()

	built := i.Build()
	body, ok := built.Payload.HTTP.Response.Body.(map[string]any)
	if !ok {
		t.Fatal("Response body is not a map")
	}
	if body["result"] != "ok" {
		t.Errorf("Body result = %v, want ok", body["result"])
	}
	if built.Payload.HTTP.Response.Headers["Content-Type"] != "application/json" {
		t.Error("Content-Type header should be set for JSON response body")
	}
}

func TestInteraction_WithMatchingRule(t *testing.T) {
	i := NewInteraction("test").
		WithHTTPRequest("GET", "/test").
		WillRespondWith(200).
		Build()
	i.WithMatchingRule("$.response.body.id", TypeMatcher())

	built := i.Build()
	if len(built.MatchingRules) != 1 {
		t.Fatalf("MatchingRules count = %d, want 1", len(built.MatchingRules))
	}
}

func TestMatchingRuleHelpers(t *testing.T) {
	tests := []struct {
		name     string
		rule     contract.MatchingRule
		wantType contract.MatchType
	}{
		{"TypeMatcher", TypeMatcher(), contract.MatchType_},
		{"IntegerMatcher", IntegerMatcher(), contract.MatchInteger},
		{"DecimalMatcher", DecimalMatcher(), contract.MatchDecimal},
		{"OptionalMatcher", OptionalMatcher(), contract.MatchOptional},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.rule.Match != tt.wantType {
				t.Errorf("Match = %s, want %s", tt.rule.Match, tt.wantType)
			}
		})
	}
}

func TestRegexMatcher(t *testing.T) {
	rule := RegexMatcher(`^\d+$`)
	if rule.Match != contract.MatchRegex {
		t.Errorf("Match = %s, want regex", rule.Match)
	}
	if rule.Pattern != `^\d+$` {
		t.Errorf("Pattern = %s, want ^\\d+$", rule.Pattern)
	}
}

func TestIncludeMatcher(t *testing.T) {
	rule := IncludeMatcher("expected")
	if rule.Match != contract.MatchInclude {
		t.Errorf("Match = %s, want include", rule.Match)
	}
	if rule.Value != "expected" {
		t.Errorf("Value = %v, want expected", rule.Value)
	}
}

func TestEachLikeMatcher(t *testing.T) {
	rule := EachLikeMatcher(3)
	if rule.Match != contract.MatchEachLike {
		t.Errorf("Match = %s, want each_like", rule.Match)
	}
	if rule.Min == nil || *rule.Min != 3 {
		t.Errorf("Min = %v, want 3", rule.Min)
	}
}

func TestContract_AddInteraction(t *testing.T) {
	interaction := NewInteraction("test").
		WithHTTPRequest("GET", "/test").
		WillRespondWith(200).
		Build()

	c := NewContract("consumer", "provider").
		AddInteraction(interaction).
		Build()

	if len(c.Interactions) != 1 {
		t.Fatalf("Interactions count = %d, want 1", len(c.Interactions))
	}
}

func TestContract_Checksum(t *testing.T) {
	c := NewContract("consumer", "provider").
		AddInteraction(NewInteraction("test").
			WithHTTPRequest("GET", "/test").
			WillRespondWith(200).
			Build()).
		Build()

	if c.Metadata.Checksum == "" {
		t.Error("Checksum should be generated")
	}
}
