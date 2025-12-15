package contract

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
)

// LoadFromFile loads a contract from a JSON file.
func LoadFromFile(path string) (*Contract, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open contract file: %w", err)
	}
	defer f.Close()

	return LoadFromReader(f)
}

// LoadFromReader loads a contract from a reader.
func LoadFromReader(r io.Reader) (*Contract, error) {
	var c Contract
	if err := json.NewDecoder(r).Decode(&c); err != nil {
		return nil, fmt.Errorf("failed to decode contract: %w", err)
	}

	// Validate the loaded contract
	if err := Validate(&c); err != nil {
		return nil, fmt.Errorf("invalid contract: %w", err)
	}

	return &c, nil
}

// LoadFromBytes loads a contract from JSON bytes.
func LoadFromBytes(data []byte) (*Contract, error) {
	var c Contract
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to decode contract: %w", err)
	}

	if err := Validate(&c); err != nil {
		return nil, fmt.Errorf("invalid contract: %w", err)
	}

	return &c, nil
}

// SaveToFile saves a contract to a JSON file.
func SaveToFile(c *Contract, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create contract file: %w", err)
	}
	defer f.Close()

	return SaveToWriter(c, f)
}

// SaveToWriter saves a contract to a writer with pretty formatting.
func SaveToWriter(c *Contract, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode contract: %w", err)
	}
	return nil
}

// SaveToBytes converts a contract to JSON bytes with pretty formatting.
func SaveToBytes(c *Contract) ([]byte, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to encode contract: %w", err)
	}
	return data, nil
}

// Validate performs validation on a contract.
func Validate(c *Contract) error {
	if c.Metadata.ID == "" {
		return fmt.Errorf("contract ID is required")
	}

	if c.Metadata.Consumer.Name == "" {
		return fmt.Errorf("consumer name is required")
	}

	if c.Metadata.Provider.Name == "" {
		return fmt.Errorf("provider name is required")
	}

	// Validate each interaction
	for i, interaction := range c.Interactions {
		if err := validateInteraction(&interaction, i); err != nil {
			return err
		}
	}

	return nil
}

func validateInteraction(interaction *Interaction, index int) error {
	if interaction.ID == "" {
		return fmt.Errorf("interaction %d: ID is required", index)
	}

	if interaction.Protocol == "" {
		return fmt.Errorf("interaction %d (%s): protocol is required", index, interaction.ID)
	}

	// Validate protocol-specific payload
	switch interaction.Protocol {
	case ProtocolHTTP:
		if interaction.Payload.HTTP == nil {
			return fmt.Errorf("interaction %d (%s): HTTP payload is required for http protocol", index, interaction.ID)
		}
		if interaction.Payload.HTTP.Request.Method == "" {
			return fmt.Errorf("interaction %d (%s): HTTP request method is required", index, interaction.ID)
		}
		if interaction.Payload.HTTP.Request.Path == "" {
			return fmt.Errorf("interaction %d (%s): HTTP request path is required", index, interaction.ID)
		}
	case ProtocolGRPC:
		if interaction.Payload.GRPC == nil {
			return fmt.Errorf("interaction %d (%s): gRPC payload is required for grpc protocol", index, interaction.ID)
		}
		if interaction.Payload.GRPC.Service == "" {
			return fmt.Errorf("interaction %d (%s): gRPC service is required", index, interaction.ID)
		}
		if interaction.Payload.GRPC.Method == "" {
			return fmt.Errorf("interaction %d (%s): gRPC method is required", index, interaction.ID)
		}
	case ProtocolAsync:
		if interaction.Payload.Async == nil {
			return fmt.Errorf("interaction %d (%s): async payload is required for async protocol", index, interaction.ID)
		}
		if interaction.Payload.Async.Destination == "" {
			return fmt.Errorf("interaction %d (%s): async destination is required", index, interaction.ID)
		}
	default:
		return fmt.Errorf("interaction %d (%s): unknown protocol: %s", index, interaction.ID, interaction.Protocol)
	}

	return nil
}

// NewContract creates a new contract with default values.
func NewContract(consumer, provider string) *Contract {
	return &Contract{
		Metadata: Metadata{
			ID:        uuid.New().String(),
			Version:   "1.0.0",
			Consumer:  Participant{Name: consumer},
			Provider:  Participant{Name: provider},
			Status:    StatusDraft,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []Interaction{},
	}
}

// Clone creates a deep copy of a contract.
func Clone(c *Contract) (*Contract, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	var clone Contract
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}

	return &clone, nil
}
