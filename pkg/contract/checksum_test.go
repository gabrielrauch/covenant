package contract

import (
	"bytes"
	"testing"
)

func TestVerifyChecksumEmpty(t *testing.T) {
	c := &Contract{Metadata: Metadata{Checksum: ""}}
	ok, err := VerifyChecksum(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected empty checksum to pass verification")
	}
}

func TestVerifyChecksumMismatch(t *testing.T) {
	c := &Contract{
		Metadata:     Metadata{Checksum: "bad"},
		Interactions: []Interaction{{ID: "a"}},
	}
	ok, err := VerifyChecksum(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected mismatched checksum to fail")
	}
}

func TestComputeChecksumDeterministic(t *testing.T) {
	interactions := []Interaction{{ID: "x", Description: "d"}}
	a, err := ComputeChecksum(interactions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := ComputeChecksum(interactions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != b {
		t.Errorf("checksums differ: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Errorf("expected 64-char hex string, got length %d", len(a))
	}
}

func TestVerifyChecksum_CorrectChecksum(t *testing.T) {
	interactions := []Interaction{{ID: "int-1", Description: "test"}}
	checksum, err := ComputeChecksum(interactions)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}

	c := &Contract{
		Metadata:     Metadata{Checksum: checksum},
		Interactions: interactions,
	}
	ok, err := VerifyChecksum(c)
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}
	if !ok {
		t.Error("expected correct checksum to pass verification")
	}
}

func TestUpdateChecksum(t *testing.T) {
	t.Run("sets checksum on contract", func(t *testing.T) {
		c := &Contract{
			Metadata: Metadata{},
			Interactions: []Interaction{
				{ID: "i1", Description: "first"},
				{ID: "i2", Description: "second"},
			},
		}

		err := UpdateChecksum(c)
		if err != nil {
			t.Fatalf("UpdateChecksum failed: %v", err)
		}
		if c.Metadata.Checksum == "" {
			t.Error("expected checksum to be set")
		}
		if len(c.Metadata.Checksum) != 64 {
			t.Errorf("expected 64-char hex, got length %d", len(c.Metadata.Checksum))
		}
	})

	t.Run("checksum matches ComputeChecksum", func(t *testing.T) {
		interactions := []Interaction{{ID: "a"}, {ID: "b"}}
		c := &Contract{
			Metadata:     Metadata{},
			Interactions: interactions,
		}

		if err := UpdateChecksum(c); err != nil {
			t.Fatalf("UpdateChecksum failed: %v", err)
		}

		expected, err := ComputeChecksum(interactions)
		if err != nil {
			t.Fatalf("ComputeChecksum failed: %v", err)
		}
		if c.Metadata.Checksum != expected {
			t.Errorf("checksum = %q, want %q", c.Metadata.Checksum, expected)
		}
	})

	t.Run("empty interactions produces valid checksum", func(t *testing.T) {
		c := &Contract{
			Metadata:     Metadata{},
			Interactions: []Interaction{},
		}

		if err := UpdateChecksum(c); err != nil {
			t.Fatalf("UpdateChecksum failed: %v", err)
		}
		if c.Metadata.Checksum == "" {
			t.Error("expected non-empty checksum even for empty interactions")
		}
	})
}

func TestComputeChecksum_DifferentInputsProduceDifferentChecksums(t *testing.T) {
	a, err := ComputeChecksum([]Interaction{{ID: "a"}})
	if err != nil {
		t.Fatalf("ComputeChecksum a failed: %v", err)
	}
	b, err := ComputeChecksum([]Interaction{{ID: "b"}})
	if err != nil {
		t.Fatalf("ComputeChecksum b failed: %v", err)
	}
	if a == b {
		t.Error("expected different checksums for different inputs")
	}
}

func TestComputeChecksum_EmptySlice(t *testing.T) {
	checksum, err := ComputeChecksum([]Interaction{})
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}
	if checksum == "" {
		t.Error("expected non-empty checksum for empty slice")
	}
	if len(checksum) != 64 {
		t.Errorf("expected 64-char hex, got length %d", len(checksum))
	}
}

func TestComputeChecksum_NilSlice(t *testing.T) {
	checksum, err := ComputeChecksum(nil)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}
	if checksum == "" {
		t.Error("expected non-empty checksum for nil slice")
	}
}

//nolint:gocyclo // test function
func TestCanonicalJSON_SortedKeys(t *testing.T) {
	t.Run("simple struct", func(t *testing.T) {
		input := map[string]any{"b": 2, "a": 1}
		data, err := CanonicalJSON(input)
		if err != nil {
			t.Fatalf("CanonicalJSON failed: %v", err)
		}
		expected := `{"a":1,"b":2}`
		if string(data) != expected {
			t.Errorf("got %s, want %s", string(data), expected)
		}
	})

	t.Run("nested maps have sorted keys", func(t *testing.T) {
		input := map[string]any{
			"z": map[string]any{"b": 2, "a": 1},
			"a": "first",
		}
		data, err := CanonicalJSON(input)
		if err != nil {
			t.Fatalf("CanonicalJSON failed: %v", err)
		}
		got := string(data)
		// "a" should come before "z"
		if got[1] != '"' || got[2] != 'a' {
			t.Errorf("expected 'a' key first in %s", got)
		}
	})

	t.Run("arrays are preserved in order", func(t *testing.T) {
		input := []any{3, 1, 2}
		data, err := CanonicalJSON(input)
		if err != nil {
			t.Fatalf("CanonicalJSON failed: %v", err)
		}
		expected := `[3,1,2]`
		if string(data) != expected {
			t.Errorf("got %s, want %s", string(data), expected)
		}
	})

	t.Run("scalar values pass through", func(t *testing.T) {
		data, err := CanonicalJSON("hello")
		if err != nil {
			t.Fatalf("CanonicalJSON failed: %v", err)
		}
		expected := `"hello"`
		if string(data) != expected {
			t.Errorf("got %s, want %s", string(data), expected)
		}
	})

	t.Run("deterministic for same input", func(t *testing.T) {
		input := map[string]any{"z": 26, "a": 1, "m": 13}
		d1, err1 := CanonicalJSON(input)
		d2, err2 := CanonicalJSON(input)
		if err1 != nil || err2 != nil {
			t.Fatalf("CanonicalJSON failed: %v / %v", err1, err2)
		}
		if !bytes.Equal(d1, d2) {
			t.Errorf("non-deterministic: %s vs %s", d1, d2)
		}
	})

	t.Run("nested array with maps", func(t *testing.T) {
		input := []any{
			map[string]any{"b": 2, "a": 1},
			map[string]any{"d": 4, "c": 3},
		}
		data, err := CanonicalJSON(input)
		if err != nil {
			t.Fatalf("CanonicalJSON failed: %v", err)
		}
		expected := `[{"a":1,"b":2},{"c":3,"d":4}]`
		if string(data) != expected {
			t.Errorf("got %s, want %s", string(data), expected)
		}
	})

	t.Run("empty map", func(t *testing.T) {
		data, err := CanonicalJSON(map[string]any{})
		if err != nil {
			t.Fatalf("CanonicalJSON failed: %v", err)
		}
		if string(data) != "{}" {
			t.Errorf("got %s, want {}", string(data))
		}
	})
}

func TestVerifyChecksum_RoundTrip(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{},
		Interactions: []Interaction{
			{ID: "i1", Description: "desc1", Protocol: ProtocolHTTP},
			{ID: "i2", Description: "desc2", Protocol: ProtocolGRPC},
		},
	}

	if err := UpdateChecksum(c); err != nil {
		t.Fatalf("UpdateChecksum failed: %v", err)
	}

	ok, err := VerifyChecksum(c)
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}
	if !ok {
		t.Error("expected round-trip checksum verification to pass")
	}

	// Modify interactions and verify checksum no longer matches
	c.Interactions[0].Description = "modified"
	ok2, err := VerifyChecksum(c)
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}
	if ok2 {
		t.Error("expected checksum to fail after modifying interactions")
	}
}
