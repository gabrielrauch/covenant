package contract

import (
	"encoding/json"
	"testing"
)

func TestPayload_MarshalJSON(t *testing.T) {
	t.Run("empty payload", func(t *testing.T) {
		p := Payload{}
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "{}" {
			t.Errorf("got %s, want {}", data)
		}
	})
}

func TestPayload_UnmarshalJSON(t *testing.T) {
	t.Run("grpc and async together", func(t *testing.T) {
		raw := `{"grpc":{"service":"s","method":"m","request":{"message":{}},"response":{"status":"OK","message":{}}},"async":{"destination":"d","direction":"publish","message":{"payload":{}}}}`
		var p Payload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.GRPC == nil {
			t.Error("expected GRPC to be set")
		}
		if p.Async == nil {
			t.Error("expected Async to be set")
		}
	})
}
