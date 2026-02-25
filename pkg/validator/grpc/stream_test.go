package grpc

import (
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

// newRecorderWithMessages creates a StreamRecorder pre-populated with the given messages.
func newRecorderWithMessages(t *testing.T, msgs []StreamMessage) *StreamRecorder {
	t.Helper()
	r := NewStreamRecorder()
	for _, m := range msgs {
		switch m.Direction {
		case contract.StreamDirectionClient:
			r.RecordClientMessage(m.Data, m.Metadata)
		case contract.StreamDirectionServer:
			r.RecordServerMessage(m.Data, m.Metadata)
		}
	}
	return r
}

func TestStreamRecorder_RecordClientMessage(t *testing.T) {
	r := NewStreamRecorder()
	r.RecordClientMessage([]byte(`{"id":1}`), map[string]string{"key": "val"})

	msgs := r.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Direction != contract.StreamDirectionClient {
		t.Errorf("direction = %q, want %q", msgs[0].Direction, contract.StreamDirectionClient)
	}
	if string(msgs[0].Data) != `{"id":1}` {
		t.Errorf("data = %q, want %q", string(msgs[0].Data), `{"id":1}`)
	}
	if msgs[0].Metadata["key"] != "val" {
		t.Errorf("metadata[key] = %q, want %q", msgs[0].Metadata["key"], "val")
	}
}

func TestStreamRecorder_RecordServerMessage(t *testing.T) {
	r := NewStreamRecorder()
	r.RecordServerMessage([]byte(`{"ok":true}`), nil)

	msgs := r.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Direction != contract.StreamDirectionServer {
		t.Errorf("direction = %q, want %q", msgs[0].Direction, contract.StreamDirectionServer)
	}
	if string(msgs[0].Data) != `{"ok":true}` {
		t.Errorf("data = %q, want %q", string(msgs[0].Data), `{"ok":true}`)
	}
	if msgs[0].Metadata != nil {
		t.Errorf("metadata = %v, want nil", msgs[0].Metadata)
	}
}

func TestStreamRecorder_ClientMessages(t *testing.T) {
	tests := []struct {
		name      string
		messages  []StreamMessage
		wantCount int
	}{
		{
			name:      "empty recorder",
			messages:  nil,
			wantCount: 0,
		},
		{
			name: "only client messages",
			messages: []StreamMessage{
				{Direction: contract.StreamDirectionClient, Data: []byte(`{"a":1}`)},
				{Direction: contract.StreamDirectionClient, Data: []byte(`{"a":2}`)},
			},
			wantCount: 2,
		},
		{
			name: "only server messages",
			messages: []StreamMessage{
				{Direction: contract.StreamDirectionServer, Data: []byte(`{"b":1}`)},
			},
			wantCount: 0,
		},
		{
			name: "mixed messages",
			messages: []StreamMessage{
				{Direction: contract.StreamDirectionClient, Data: []byte(`{"c":1}`)},
				{Direction: contract.StreamDirectionServer, Data: []byte(`{"c":2}`)},
				{Direction: contract.StreamDirectionClient, Data: []byte(`{"c":3}`)},
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRecorderWithMessages(t, tt.messages)
			got := r.ClientMessages()
			if len(got) != tt.wantCount {
				t.Errorf("ClientMessages() returned %d messages, want %d", len(got), tt.wantCount)
			}
			for _, m := range got {
				if m.Direction != contract.StreamDirectionClient {
					t.Errorf("ClientMessages() returned message with direction %q", m.Direction)
				}
			}
		})
	}
}

func TestStreamRecorder_ServerMessages(t *testing.T) {
	tests := []struct {
		name      string
		messages  []StreamMessage
		wantCount int
	}{
		{
			name:      "empty recorder",
			messages:  nil,
			wantCount: 0,
		},
		{
			name: "only server messages",
			messages: []StreamMessage{
				{Direction: contract.StreamDirectionServer, Data: []byte(`{"s":1}`)},
				{Direction: contract.StreamDirectionServer, Data: []byte(`{"s":2}`)},
			},
			wantCount: 2,
		},
		{
			name: "only client messages",
			messages: []StreamMessage{
				{Direction: contract.StreamDirectionClient, Data: []byte(`{"c":1}`)},
			},
			wantCount: 0,
		},
		{
			name: "mixed messages",
			messages: []StreamMessage{
				{Direction: contract.StreamDirectionServer, Data: []byte(`{"m":1}`)},
				{Direction: contract.StreamDirectionClient, Data: []byte(`{"m":2}`)},
				{Direction: contract.StreamDirectionServer, Data: []byte(`{"m":3}`)},
				{Direction: contract.StreamDirectionServer, Data: []byte(`{"m":4}`)},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRecorderWithMessages(t, tt.messages)
			got := r.ServerMessages()
			if len(got) != tt.wantCount {
				t.Errorf("ServerMessages() returned %d messages, want %d", len(got), tt.wantCount)
			}
			for _, m := range got {
				if m.Direction != contract.StreamDirectionServer {
					t.Errorf("ServerMessages() returned message with direction %q", m.Direction)
				}
			}
		})
	}
}

func TestStreamRecorder_Count(t *testing.T) {
	tests := []struct {
		name      string
		messages  []StreamMessage
		wantCount int
	}{
		{
			name:      "empty",
			messages:  nil,
			wantCount: 0,
		},
		{
			name: "single message",
			messages: []StreamMessage{
				{Direction: contract.StreamDirectionClient, Data: []byte(`{}`)},
			},
			wantCount: 1,
		},
		{
			name: "multiple mixed messages",
			messages: []StreamMessage{
				{Direction: contract.StreamDirectionClient, Data: []byte(`{}`)},
				{Direction: contract.StreamDirectionServer, Data: []byte(`{}`)},
				{Direction: contract.StreamDirectionClient, Data: []byte(`{}`)},
				{Direction: contract.StreamDirectionServer, Data: []byte(`{}`)},
				{Direction: contract.StreamDirectionServer, Data: []byte(`{}`)},
			},
			wantCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRecorderWithMessages(t, tt.messages)
			if got := r.Count(); got != tt.wantCount {
				t.Errorf("Count() = %d, want %d", got, tt.wantCount)
			}
		})
	}
}

func TestStreamRecorder_Messages_PreservesOrder(t *testing.T) {
	r := NewStreamRecorder()
	r.RecordClientMessage([]byte(`{"seq":1}`), nil)
	r.RecordServerMessage([]byte(`{"seq":2}`), nil)
	r.RecordClientMessage([]byte(`{"seq":3}`), nil)

	msgs := r.Messages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	wantDirections := []contract.StreamDirection{
		contract.StreamDirectionClient,
		contract.StreamDirectionServer,
		contract.StreamDirectionClient,
	}
	wantData := []string{`{"seq":1}`, `{"seq":2}`, `{"seq":3}`}

	for i, m := range msgs {
		if m.Direction != wantDirections[i] {
			t.Errorf("message %d: direction = %q, want %q", i, m.Direction, wantDirections[i])
		}
		if string(m.Data) != wantData[i] {
			t.Errorf("message %d: data = %q, want %q", i, string(m.Data), wantData[i])
		}
	}
}

// buildStreamMatcher creates a StreamMatcher with the given expected and actual messages.
func buildStreamMatcher(t *testing.T, expected []contract.StreamMessage, actual []StreamMessage) *StreamMatcher {
	t.Helper()
	m := NewStreamMatcher(expected)
	for _, a := range actual {
		m.AddActual(a)
	}
	return m
}

func TestStreamMatcher_LengthMismatch(t *testing.T) {
	tests := []struct {
		name         string
		expected     []contract.StreamMessage
		actual       []StreamMessage
		wantMatched  bool
		wantErrCount int
	}{
		{
			name: "more expected than actual",
			expected: []contract.StreamMessage{
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionServer},
			},
			actual: []StreamMessage{
				{Direction: contract.StreamDirectionClient},
			},
			wantMatched:  false,
			wantErrCount: 1,
		},
		{
			name: "more actual than expected",
			expected: []contract.StreamMessage{
				{Direction: contract.StreamDirectionClient},
			},
			actual: []StreamMessage{
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionServer},
			},
			wantMatched:  false,
			wantErrCount: 1,
		},
		{
			name:         "both empty",
			expected:     nil,
			actual:       nil,
			wantMatched:  true,
			wantErrCount: 0,
		},
		{
			name:         "expected non-empty actual empty",
			expected:     []contract.StreamMessage{{Direction: contract.StreamDirectionClient}},
			actual:       nil,
			wantMatched:  false,
			wantErrCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := buildStreamMatcher(t, tt.expected, tt.actual)
			matched, errs := m.Match()
			if matched != tt.wantMatched {
				t.Errorf("Match() matched = %v, want %v", matched, tt.wantMatched)
			}
			if len(errs) != tt.wantErrCount {
				t.Errorf("Match() returned %d errors, want %d: %v", len(errs), tt.wantErrCount, errs)
			}
		})
	}
}

func TestStreamMatcher_DirectionMismatch(t *testing.T) {
	tests := []struct {
		name         string
		expected     []contract.StreamMessage
		actual       []StreamMessage
		wantMatched  bool
		wantErrCount int
	}{
		{
			name: "single direction mismatch",
			expected: []contract.StreamMessage{
				{Direction: contract.StreamDirectionClient},
			},
			actual: []StreamMessage{
				{Direction: contract.StreamDirectionServer},
			},
			wantMatched:  false,
			wantErrCount: 1,
		},
		{
			name: "multiple direction mismatches",
			expected: []contract.StreamMessage{
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionServer},
			},
			actual: []StreamMessage{
				{Direction: contract.StreamDirectionServer},
				{Direction: contract.StreamDirectionClient},
			},
			wantMatched:  false,
			wantErrCount: 2,
		},
		{
			name: "partial mismatch in sequence",
			expected: []contract.StreamMessage{
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionServer},
				{Direction: contract.StreamDirectionClient},
			},
			actual: []StreamMessage{
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionClient},
			},
			wantMatched:  false,
			wantErrCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := buildStreamMatcher(t, tt.expected, tt.actual)
			matched, errs := m.Match()
			if matched != tt.wantMatched {
				t.Errorf("Match() matched = %v, want %v", matched, tt.wantMatched)
			}
			if len(errs) != tt.wantErrCount {
				t.Errorf("Match() returned %d errors, want %d: %v", len(errs), tt.wantErrCount, errs)
			}
		})
	}
}

func TestStreamMatcher_AllMatching(t *testing.T) {
	tests := []struct {
		name     string
		expected []contract.StreamMessage
		actual   []StreamMessage
	}{
		{
			name:     "empty sequences",
			expected: nil,
			actual:   nil,
		},
		{
			name: "single matching message",
			expected: []contract.StreamMessage{
				{Direction: contract.StreamDirectionClient},
			},
			actual: []StreamMessage{
				{Direction: contract.StreamDirectionClient},
			},
		},
		{
			name: "full bidirectional sequence",
			expected: []contract.StreamMessage{
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionServer},
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionServer},
			},
			actual: []StreamMessage{
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionServer},
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionServer},
			},
		},
		{
			name: "all client messages",
			expected: []contract.StreamMessage{
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionClient},
			},
			actual: []StreamMessage{
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionClient},
				{Direction: contract.StreamDirectionClient},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := buildStreamMatcher(t, tt.expected, tt.actual)
			matched, errs := m.Match()
			if !matched {
				t.Errorf("Match() should succeed, got errors: %v", errs)
			}
			if len(errs) != 0 {
				t.Errorf("Match() returned %d errors, want 0: %v", len(errs), errs)
			}
		})
	}
}

func TestStreamMatcher_LengthAndDirectionMismatch(t *testing.T) {
	// When both length AND direction are wrong, we get errors for both.
	expected := []contract.StreamMessage{
		{Direction: contract.StreamDirectionClient},
		{Direction: contract.StreamDirectionServer},
		{Direction: contract.StreamDirectionClient},
	}
	actual := []StreamMessage{
		{Direction: contract.StreamDirectionServer}, // direction mismatch
	}

	m := buildStreamMatcher(t, expected, actual)
	matched, errs := m.Match()
	if matched {
		t.Error("Match() should fail with both length and direction mismatches")
	}
	// Expect 1 length mismatch + 1 direction mismatch (only first element compared)
	if len(errs) != 2 {
		t.Errorf("Match() returned %d errors, want 2: %v", len(errs), errs)
	}
}

func TestStreamExpectation_ValidateCount(t *testing.T) {
	tests := []struct {
		name  string
		exp   StreamExpectation
		count int
		want  bool
	}{
		{
			name:  "within min and max bounds",
			exp:   StreamExpectation{MinMessages: 2, MaxMessages: 5},
			count: 3,
			want:  true,
		},
		{
			name:  "at min boundary",
			exp:   StreamExpectation{MinMessages: 2, MaxMessages: 5},
			count: 2,
			want:  true,
		},
		{
			name:  "at max boundary",
			exp:   StreamExpectation{MinMessages: 2, MaxMessages: 5},
			count: 5,
			want:  true,
		},
		{
			name:  "below min",
			exp:   StreamExpectation{MinMessages: 3, MaxMessages: 10},
			count: 2,
			want:  false,
		},
		{
			name:  "above max",
			exp:   StreamExpectation{MinMessages: 1, MaxMessages: 3},
			count: 4,
			want:  false,
		},
		{
			name:  "zero count with min set",
			exp:   StreamExpectation{MinMessages: 1, MaxMessages: 10},
			count: 0,
			want:  false,
		},
		{
			name:  "unbounded min and max",
			exp:   StreamExpectation{MinMessages: 0, MaxMessages: 0},
			count: 100,
			want:  true,
		},
		{
			name:  "unbounded min and max with zero count",
			exp:   StreamExpectation{MinMessages: 0, MaxMessages: 0},
			count: 0,
			want:  true,
		},
		{
			name:  "min only no max",
			exp:   StreamExpectation{MinMessages: 2, MaxMessages: 0},
			count: 1000,
			want:  true,
		},
		{
			name:  "min only below min",
			exp:   StreamExpectation{MinMessages: 5, MaxMessages: 0},
			count: 3,
			want:  false,
		},
		{
			name:  "max only no min",
			exp:   StreamExpectation{MinMessages: 0, MaxMessages: 10},
			count: 0,
			want:  true,
		},
		{
			name:  "max only above max",
			exp:   StreamExpectation{MinMessages: 0, MaxMessages: 3},
			count: 5,
			want:  false,
		},
		{
			name:  "max only at max",
			exp:   StreamExpectation{MinMessages: 0, MaxMessages: 3},
			count: 3,
			want:  true,
		},
		{
			name:  "min equals max exact count",
			exp:   StreamExpectation{MinMessages: 5, MaxMessages: 5},
			count: 5,
			want:  true,
		},
		{
			name:  "min equals max wrong count",
			exp:   StreamExpectation{MinMessages: 5, MaxMessages: 5},
			count: 4,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.exp.ValidateCount(tt.count)
			if got != tt.want {
				t.Errorf("ValidateCount(%d) = %v, want %v (min=%d, max=%d)",
					tt.count, got, tt.want, tt.exp.MinMessages, tt.exp.MaxMessages)
			}
		})
	}
}

func TestStreamExpectation_Ordered(t *testing.T) {
	// Ensure the Ordered field is settable and accessible.
	exp := StreamExpectation{
		Ordered: true,
	}
	if !exp.Ordered {
		t.Error("expected Ordered to be true")
	}
}

func TestNewStreamRecorder(t *testing.T) {
	r := NewStreamRecorder()
	if r == nil {
		t.Fatal("NewStreamRecorder returned nil")
	}
	if r.Count() != 0 {
		t.Errorf("new recorder Count() = %d, want 0", r.Count())
	}
	if len(r.Messages()) != 0 {
		t.Errorf("new recorder Messages() has %d items, want 0", len(r.Messages()))
	}
}

func TestNewStreamMatcher(t *testing.T) {
	expected := []contract.StreamMessage{
		{Direction: contract.StreamDirectionClient},
	}
	m := NewStreamMatcher(expected)
	if m == nil {
		t.Fatal("NewStreamMatcher returned nil")
	}
}

func TestStreamMatcher_AddActual(t *testing.T) {
	m := NewStreamMatcher(nil)
	m.AddActual(StreamMessage{Direction: contract.StreamDirectionClient, Data: []byte(`{}`)})
	m.AddActual(StreamMessage{Direction: contract.StreamDirectionServer, Data: []byte(`{}`)})

	// After adding 2 actual messages against nil expected, length mismatch is expected.
	matched, errs := m.Match()
	if matched {
		t.Error("Match() should fail when expected is nil but actual has messages")
	}
	if len(errs) == 0 {
		t.Error("Match() should return errors for length mismatch")
	}
}
