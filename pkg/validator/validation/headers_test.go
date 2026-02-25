package validation

import (
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

func TestValidateHeaders_AllPresent(t *testing.T) {
	expected := map[string]string{
		"Content-Type": "application/json",
		"X-Request-ID": "123",
	}
	actual := map[string]string{
		"Content-Type": "application/json",
		"X-Request-ID": "123",
	}

	errors := ValidateHeaders(expected, actual, nil, DefaultHTTPHeaderConfig("$.response.headers"))
	if len(errors) != 0 {
		t.Errorf("expected no errors, got: %v", errors)
	}
}

func TestValidateHeaders_MissingHeader(t *testing.T) {
	expected := map[string]string{
		"Content-Type": "application/json",
		"X-Request-ID": "123",
	}
	actual := map[string]string{
		"Content-Type": "application/json",
	}

	errors := ValidateHeaders(expected, actual, nil, DefaultHTTPHeaderConfig("$.response.headers"))
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got: %d", len(errors))
	}
	if errors[0].Path != "$.response.headers.X-Request-ID" {
		t.Errorf("expected path $.response.headers.X-Request-ID, got: %s", errors[0].Path)
	}
	if errors[0].Actual != "(missing)" {
		t.Errorf("expected actual '(missing)', got: %s", errors[0].Actual)
	}
}

func TestValidateHeaders_ValueMismatch(t *testing.T) {
	expected := map[string]string{
		"Content-Type": "application/json",
	}
	actual := map[string]string{
		"Content-Type": "text/plain",
	}

	errors := ValidateHeaders(expected, actual, nil, DefaultHTTPHeaderConfig("$.response.headers"))
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got: %d", len(errors))
	}
	if errors[0].Expected != "application/json" {
		t.Errorf("expected Expected='application/json', got: %s", errors[0].Expected)
	}
	if errors[0].Actual != "text/plain" {
		t.Errorf("expected Actual='text/plain', got: %s", errors[0].Actual)
	}
}

func TestValidateHeaders_WithMatchingRule(t *testing.T) {
	expected := map[string]string{
		"X-Request-ID": "sample-id",
	}
	actual := map[string]string{
		"X-Request-ID": "different-id",
	}
	rules := contract.MatchingRules{
		"$.response.headers.X-Request-ID": {Match: contract.MatchTypeValue},
	}

	errors := ValidateHeaders(expected, actual, rules, DefaultHTTPHeaderConfig("$.response.headers"))
	if len(errors) != 0 {
		t.Errorf("expected no errors when matching rule exists, got: %v", errors)
	}
}

func TestValidateHeaders_CaseInsensitiveLookup(t *testing.T) {
	expected := map[string]string{
		"Content-Type": "application/json",
	}
	actual := map[string]string{
		"content-type": "application/json", // lowercase
	}

	// HTTP headers should be case-insensitive
	errors := ValidateHeaders(expected, actual, nil, DefaultHTTPHeaderConfig("$.response.headers"))
	if len(errors) != 0 {
		t.Errorf("expected case-insensitive match for HTTP headers, got errors: %v", errors)
	}
}

func TestValidateHeaders_CaseSensitiveLookup(t *testing.T) {
	expected := map[string]string{
		"Content-Type": "application/json",
	}
	actual := map[string]string{
		"content-type": "application/json", // lowercase
	}

	// gRPC metadata is case-sensitive
	config := DefaultGRPCMetadataConfig("$.response.metadata")
	errors := ValidateHeaders(expected, actual, nil, config)
	if len(errors) != 1 {
		t.Errorf("expected case-sensitive mismatch for gRPC metadata, got %d errors", len(errors))
	}
}

func TestValidateHeaders_EmptyExpected(t *testing.T) {
	expected := map[string]string{}
	actual := map[string]string{
		"Content-Type": "application/json",
	}

	errors := ValidateHeaders(expected, actual, nil, DefaultHTTPHeaderConfig("$.response.headers"))
	if len(errors) != 0 {
		t.Errorf("expected no errors for empty expected, got: %v", errors)
	}
}

func TestValidateHeaders_NilActual(t *testing.T) {
	expected := map[string]string{
		"Content-Type": "application/json",
	}

	errors := ValidateHeaders(expected, nil, nil, DefaultHTTPHeaderConfig("$.response.headers"))
	if len(errors) != 0 {
		t.Errorf("expected no errors for nil actual, got: %v", errors)
	}
}

func TestValidateHeaders_CustomFieldName(t *testing.T) {
	expected := map[string]string{
		"key": "value",
	}
	actual := map[string]string{}

	config := HeaderValidationConfig{
		BasePath:  "$.message.headers",
		FieldName: "custom-field",
	}

	errors := ValidateHeaders(expected, actual, nil, config)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got: %d", len(errors))
	}
	if errors[0].Message != `expected custom-field key to be present` {
		t.Errorf("expected custom field name in message, got: %s", errors[0].Message)
	}
}

func TestValidateHeaders_DefaultFieldName(t *testing.T) {
	expected := map[string]string{"k": "v"}
	actual := map[string]string{}

	errors := ValidateHeaders(expected, actual, nil, HeaderValidationConfig{BasePath: "$.p"})
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if errors[0].Message != `expected header k to be present` {
		t.Errorf("unexpected message: %s", errors[0].Message)
	}
}
