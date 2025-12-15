package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// ComputeChecksum calculates the SHA256 checksum of the interactions array.
// The checksum is computed from the canonical JSON representation (sorted keys, no whitespace).
func ComputeChecksum(interactions []Interaction) (string, error) {
	canonical, err := CanonicalJSON(interactions)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]), nil
}

// CanonicalJSON produces a canonical JSON representation suitable for checksumming.
// Keys are sorted alphabetically and no extra whitespace is included.
func CanonicalJSON(v any) ([]byte, error) {
	// First marshal to get the raw JSON
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// Unmarshal into a generic structure
	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil, err
	}

	// Canonicalize the structure
	canonical := canonicalize(generic)

	// Marshal without any extra whitespace
	return json.Marshal(canonical)
}

// canonicalize recursively sorts map keys and returns a canonical representation.
func canonicalize(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return canonicalizeMap(val)
	case []any:
		return canonicalizeSlice(val)
	default:
		return v
	}
}

func canonicalizeMap(m map[string]any) any {
	// Get sorted keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Create an ordered map representation
	// Since Go's json.Marshal doesn't guarantee order, we use a slice of key-value pairs
	// wrapped in a custom type that marshals in order
	result := make(map[string]any, len(m))
	for _, k := range keys {
		result[k] = canonicalize(m[k])
	}
	return &orderedMap{keys: keys, values: result}
}

func canonicalizeSlice(s []any) any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = canonicalize(v)
	}
	return result
}

// orderedMap is a map that marshals with keys in a specific order.
type orderedMap struct {
	keys   []string
	values map[string]any
}

// MarshalJSON implements json.Marshaler for orderedMap.
func (om *orderedMap) MarshalJSON() ([]byte, error) {
	// Build JSON manually to ensure key order
	buf := []byte{'{'}

	for i, k := range om.keys {
		if i > 0 {
			buf = append(buf, ',')
		}

		// Marshal the key
		keyBytes, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf = append(buf, keyBytes...)
		buf = append(buf, ':')

		// Marshal the value
		valBytes, err := json.Marshal(om.values[k])
		if err != nil {
			return nil, err
		}
		buf = append(buf, valBytes...)
	}

	buf = append(buf, '}')
	return buf, nil
}

// VerifyChecksum verifies that a contract's checksum matches its interactions.
func VerifyChecksum(c *Contract) (bool, error) {
	if c.Metadata.Checksum == "" {
		return true, nil // No checksum to verify
	}

	computed, err := ComputeChecksum(c.Interactions)
	if err != nil {
		return false, err
	}

	return computed == c.Metadata.Checksum, nil
}

// UpdateChecksum computes and updates the contract's checksum.
func UpdateChecksum(c *Contract) error {
	checksum, err := ComputeChecksum(c.Interactions)
	if err != nil {
		return err
	}
	c.Metadata.Checksum = checksum
	return nil
}
