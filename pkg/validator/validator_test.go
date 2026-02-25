package validator

import (
	"testing"
)

func TestNewSuccessResult(t *testing.T) {
	r := NewSuccessResult()

	if !r.Success {
		t.Error("expected Success to be true")
	}
	if len(r.Errors) != 0 {
		t.Errorf("expected no errors, got %d", len(r.Errors))
	}
}

func TestNewFailureResult(t *testing.T) {
	tests := []struct {
		name       string
		errors     []ValidationError
		wantCount  int
		wantFields []ValidationError
	}{
		{
			name:      "no errors provided",
			errors:    nil,
			wantCount: 0,
		},
		{
			name: "single error",
			errors: []ValidationError{
				{
					Path:     "$.body.name",
					Expected: "string",
					Actual:   "42",
					Rule:     "type",
					Message:  "expected string, got number",
				},
			},
			wantCount: 1,
			wantFields: []ValidationError{
				{
					Path:     "$.body.name",
					Expected: "string",
					Actual:   "42",
					Rule:     "type",
					Message:  "expected string, got number",
				},
			},
		},
		{
			name: "multiple errors",
			errors: []ValidationError{
				{Path: "$.status", Rule: "exact", Expected: "200", Actual: "404"},
				{Path: "$.body.id", Rule: "type", Expected: "integer", Actual: "abc"},
				{Path: "$.headers.Content-Type", Rule: "regex", Expected: "application/json.*", Actual: "text/plain"},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewFailureResult(tt.errors...)

			if r.Success {
				t.Error("expected Success to be false")
			}
			if len(r.Errors) != tt.wantCount {
				t.Errorf("expected %d errors, got %d", tt.wantCount, len(r.Errors))
			}

			for i, want := range tt.wantFields {
				got := r.Errors[i]
				if got.Path != want.Path {
					t.Errorf("error[%d].Path = %q, want %q", i, got.Path, want.Path)
				}
				if got.Expected != want.Expected {
					t.Errorf("error[%d].Expected = %q, want %q", i, got.Expected, want.Expected)
				}
				if got.Actual != want.Actual {
					t.Errorf("error[%d].Actual = %q, want %q", i, got.Actual, want.Actual)
				}
				if got.Rule != want.Rule {
					t.Errorf("error[%d].Rule = %q, want %q", i, got.Rule, want.Rule)
				}
				if got.Message != want.Message {
					t.Errorf("error[%d].Message = %q, want %q", i, got.Message, want.Message)
				}
			}
		})
	}
}

func TestValidationResult_AddError(t *testing.T) {
	tests := []struct {
		name         string
		initial      ValidationResult
		addErr       *ValidationError
		wantSuccess  bool
		wantErrCount int
		wantLastPath string
	}{
		{
			name:    "add error to success result marks it failed",
			initial: NewSuccessResult(),
			addErr: &ValidationError{
				Path:    "$.body.id",
				Rule:    "exact",
				Message: "value mismatch",
			},
			wantSuccess:  false,
			wantErrCount: 1,
			wantLastPath: "$.body.id",
		},
		{
			name: "add error to already failed result stays failed",
			initial: NewFailureResult(ValidationError{
				Path: "$.status",
				Rule: "exact",
			}),
			addErr: &ValidationError{
				Path:    "$.body.name",
				Rule:    "type",
				Message: "type mismatch",
			},
			wantSuccess:  false,
			wantErrCount: 2,
			wantLastPath: "$.body.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.initial
			r.AddError(tt.addErr)

			if r.Success != tt.wantSuccess {
				t.Errorf("Success = %v, want %v", r.Success, tt.wantSuccess)
			}
			if len(r.Errors) != tt.wantErrCount {
				t.Errorf("error count = %d, want %d", len(r.Errors), tt.wantErrCount)
			}
			last := r.Errors[len(r.Errors)-1]
			if last.Path != tt.wantLastPath {
				t.Errorf("last error Path = %q, want %q", last.Path, tt.wantLastPath)
			}
		})
	}
}

func TestValidationResult_AddError_CopiesValue(t *testing.T) {
	r := NewSuccessResult()
	err := &ValidationError{
		Path:    "$.original",
		Message: "original message",
	}

	r.AddError(err)

	// Mutating the original pointer should not affect the stored error,
	// because AddError dereferences the pointer.
	err.Path = "$.mutated"
	err.Message = "mutated message"

	if r.Errors[0].Path != "$.original" {
		t.Errorf("stored error Path was mutated: got %q, want %q", r.Errors[0].Path, "$.original")
	}
	if r.Errors[0].Message != "original message" {
		t.Errorf("stored error Message was mutated: got %q, want %q", r.Errors[0].Message, "original message")
	}
}

func TestValidationResult_Merge(t *testing.T) {
	tests := []struct {
		name         string
		base         ValidationResult
		other        ValidationResult
		wantSuccess  bool
		wantErrCount int
	}{
		{
			name:         "merge two successes stays successful",
			base:         NewSuccessResult(),
			other:        NewSuccessResult(),
			wantSuccess:  true,
			wantErrCount: 0,
		},
		{
			name: "merge failure into success marks result failed",
			base: NewSuccessResult(),
			other: NewFailureResult(
				ValidationError{Path: "$.body.id", Rule: "type"},
			),
			wantSuccess:  false,
			wantErrCount: 1,
		},
		{
			name: "merge success into failure keeps result failed",
			base: NewFailureResult(
				ValidationError{Path: "$.status", Rule: "exact"},
			),
			other:        NewSuccessResult(),
			wantSuccess:  false,
			wantErrCount: 1,
		},
		{
			name: "merge two failures combines errors",
			base: NewFailureResult(
				ValidationError{Path: "$.status", Rule: "exact"},
			),
			other: NewFailureResult(
				ValidationError{Path: "$.body.name", Rule: "type"},
				ValidationError{Path: "$.body.age", Rule: "integer"},
			),
			wantSuccess:  false,
			wantErrCount: 3,
		},
		{
			name: "merge failure with no errors into success still marks failed",
			base: NewSuccessResult(),
			other: ValidationResult{
				Success: false,
				Errors:  nil,
			},
			wantSuccess:  false,
			wantErrCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.base
			r.Merge(tt.other)

			if r.Success != tt.wantSuccess {
				t.Errorf("Success = %v, want %v", r.Success, tt.wantSuccess)
			}
			if len(r.Errors) != tt.wantErrCount {
				t.Errorf("error count = %d, want %d", len(r.Errors), tt.wantErrCount)
			}
		})
	}
}

func TestValidationResult_Merge_PreservesErrorOrder(t *testing.T) {
	base := NewFailureResult(
		ValidationError{Path: "$.first"},
	)
	other := NewFailureResult(
		ValidationError{Path: "$.second"},
		ValidationError{Path: "$.third"},
	)

	base.Merge(other)

	wantPaths := []string{"$.first", "$.second", "$.third"}
	if len(base.Errors) != len(wantPaths) {
		t.Fatalf("error count = %d, want %d", len(base.Errors), len(wantPaths))
	}
	for i, want := range wantPaths {
		if base.Errors[i].Path != want {
			t.Errorf("Errors[%d].Path = %q, want %q", i, base.Errors[i].Path, want)
		}
	}
}

func TestValidationError_Fields(t *testing.T) {
	err := ValidationError{
		Path:     "$.response.body.items[0].name",
		Expected: "string",
		Actual:   "123",
		Rule:     "type",
		Message:  "expected type string but got number",
	}

	if err.Path != "$.response.body.items[0].name" {
		t.Errorf("Path = %q, want %q", err.Path, "$.response.body.items[0].name")
	}
	if err.Expected != "string" {
		t.Errorf("Expected = %q, want %q", err.Expected, "string")
	}
	if err.Actual != "123" {
		t.Errorf("Actual = %q, want %q", err.Actual, "123")
	}
	if err.Rule != "type" {
		t.Errorf("Rule = %q, want %q", err.Rule, "type")
	}
	if err.Message != "expected type string but got number" {
		t.Errorf("Message = %q, want %q", err.Message, "expected type string but got number")
	}
}

func TestActualData_Fields(t *testing.T) {
	tests := []struct {
		name     string
		data     ActualData
		wantReq  string
		wantResp string
		wantMeta map[string]string
	}{
		{
			name: "http-style data",
			data: ActualData{
				Request:  []byte(`{"name":"alice"}`),
				Response: []byte(`{"id":1,"name":"alice"}`),
				Metadata: map[string]string{
					"Content-Type": "application/json",
					"Accept":       "application/json",
				},
				Status: 200,
			},
			wantReq:  `{"name":"alice"}`,
			wantResp: `{"id":1,"name":"alice"}`,
			wantMeta: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
		},
		{
			name:     "empty data",
			data:     ActualData{},
			wantMeta: nil,
		},
		{
			name: "nil metadata",
			data: ActualData{
				Request:  []byte("request"),
				Response: []byte("response"),
				Metadata: nil,
				Status:   "OK",
			},
			wantReq:  "request",
			wantResp: "response",
			wantMeta: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.data.Request) != tt.wantReq {
				t.Errorf("Request = %q, want %q", string(tt.data.Request), tt.wantReq)
			}
			if string(tt.data.Response) != tt.wantResp {
				t.Errorf("Response = %q, want %q", string(tt.data.Response), tt.wantResp)
			}

			if tt.wantMeta == nil {
				if tt.data.Metadata != nil {
					t.Errorf("Metadata = %v, want nil", tt.data.Metadata)
				}
			} else {
				if len(tt.data.Metadata) != len(tt.wantMeta) {
					t.Errorf("Metadata length = %d, want %d", len(tt.data.Metadata), len(tt.wantMeta))
				}
				for k, want := range tt.wantMeta {
					got, ok := tt.data.Metadata[k]
					if !ok {
						t.Errorf("Metadata missing key %q", k)
					} else if got != want {
						t.Errorf("Metadata[%q] = %q, want %q", k, got, want)
					}
				}
			}
		})
	}
}

func TestActualData_StatusAny(t *testing.T) {
	tests := []struct {
		name   string
		status any
	}{
		{name: "int status", status: 200},
		{name: "string status", status: "OK"},
		{name: "nil status", status: nil},
		{name: "float status", status: 3.14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := ActualData{Status: tt.status}
			if data.Status != tt.status {
				t.Errorf("Status = %v, want %v", data.Status, tt.status)
			}
		})
	}
}

func TestValidationResult_MultipleAddErrors(t *testing.T) {
	r := NewSuccessResult()

	errors := []ValidationError{
		{Path: "$.a", Rule: "exact", Expected: "1", Actual: "2", Message: "mismatch a"},
		{Path: "$.b", Rule: "type", Expected: "string", Actual: "42", Message: "mismatch b"},
		{Path: "$.c", Rule: "regex", Expected: "^[a-z]+$", Actual: "ABC", Message: "mismatch c"},
	}

	for i := range errors {
		r.AddError(&errors[i])
	}

	if r.Success {
		t.Error("expected Success to be false after adding errors")
	}
	if len(r.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(r.Errors))
	}

	for i, want := range errors {
		got := r.Errors[i]
		if got.Path != want.Path {
			t.Errorf("error[%d].Path = %q, want %q", i, got.Path, want.Path)
		}
		if got.Rule != want.Rule {
			t.Errorf("error[%d].Rule = %q, want %q", i, got.Rule, want.Rule)
		}
		if got.Message != want.Message {
			t.Errorf("error[%d].Message = %q, want %q", i, got.Message, want.Message)
		}
	}
}

func TestValidationResult_MergeChain(t *testing.T) {
	r := NewSuccessResult()

	r1 := NewFailureResult(ValidationError{Path: "$.a"})
	r2 := NewSuccessResult()
	r3 := NewFailureResult(
		ValidationError{Path: "$.b"},
		ValidationError{Path: "$.c"},
	)

	r.Merge(r1)
	r.Merge(r2)
	r.Merge(r3)

	if r.Success {
		t.Error("expected Success to be false after merging failures")
	}
	if len(r.Errors) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(r.Errors))
	}

	wantPaths := []string{"$.a", "$.b", "$.c"}
	for i, want := range wantPaths {
		if r.Errors[i].Path != want {
			t.Errorf("Errors[%d].Path = %q, want %q", i, r.Errors[i].Path, want)
		}
	}
}
