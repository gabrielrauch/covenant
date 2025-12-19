// Package consumer provides the consumer-side API for contract testing.
package consumer

import (
	"time"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/google/uuid"
)

// Contract is a fluent builder for creating contracts.
type Contract struct {
	contract *contract.Contract
}

// NewContract creates a new contract builder.
func NewContract(consumer, provider string) *Contract {
	return &Contract{
		contract: &contract.Contract{
			Metadata: contract.Metadata{
				ID:        uuid.New().String(),
				Version:   "1.0.0",
				Consumer:  contract.Participant{Name: consumer},
				Provider:  contract.Participant{Name: provider},
				Status:    contract.StatusDraft,
				CreatedAt: time.Now().UTC(),
			},
			Interactions:  []contract.Interaction{},
			MatchingRules: make(contract.MatchingRules),
		},
	}
}

// WithVersion sets the contract version.
func (c *Contract) WithVersion(version string) *Contract {
	c.contract.Metadata.Version = version
	return c
}

// WithConsumerVersion sets the consumer version.
func (c *Contract) WithConsumerVersion(version string) *Contract {
	c.contract.Metadata.Consumer.Version = version
	return c
}

// WithProviderVersion sets the provider version.
func (c *Contract) WithProviderVersion(version string) *Contract {
	c.contract.Metadata.Provider.Version = version
	return c
}

// WithTags adds tags to the contract.
func (c *Contract) WithTags(tags ...string) *Contract {
	c.contract.Metadata.Tags = append(c.contract.Metadata.Tags, tags...)
	return c
}

// WithMatchingRule adds a contract-level matching rule.
func (c *Contract) WithMatchingRule(path string, rule contract.MatchingRule) *Contract {
	c.contract.MatchingRules[path] = rule
	return c
}

// AddInteraction adds an interaction to the contract.
func (c *Contract) AddInteraction(interaction *Interaction) *Contract {
	c.contract.Interactions = append(c.contract.Interactions, interaction.Build())
	return c
}

// Build returns the completed contract.
func (c *Contract) Build() *contract.Contract {
	contract.UpdateChecksum(c.contract)
	return c.contract
}

// Interaction is a fluent builder for creating interactions.
type Interaction struct {
	interaction contract.Interaction
}

// NewInteraction creates a new interaction builder.
func NewInteraction(description string) *Interaction {
	return &Interaction{
		interaction: contract.Interaction{
			ID:          uuid.New().String(),
			Description: description,
		},
	}
}

// WithID sets a custom ID for the interaction.
func (i *Interaction) WithID(id string) *Interaction {
	i.interaction.ID = id
	return i
}

// Given adds a provider state to the interaction.
func (i *Interaction) Given(name string, params map[string]any) *Interaction {
	i.interaction.ProviderStates = append(i.interaction.ProviderStates, contract.ProviderState{
		Name:   name,
		Params: params,
	})
	return i
}

// WithHTTPRequest sets up an HTTP interaction with the given request.
func (i *Interaction) WithHTTPRequest(method, path string) *HTTPRequestBuilder {
	i.interaction.Protocol = contract.ProtocolHTTP
	i.interaction.Payload.HTTP = &contract.HTTPPayload{
		Request: contract.HTTPRequest{
			Method: method,
			Path:   path,
		},
	}
	return &HTTPRequestBuilder{interaction: i}
}

// WithMatchingRule adds an interaction-level matching rule.
func (i *Interaction) WithMatchingRule(path string, rule contract.MatchingRule) *Interaction {
	if i.interaction.MatchingRules == nil {
		i.interaction.MatchingRules = make(contract.MatchingRules)
	}
	i.interaction.MatchingRules[path] = rule
	return i
}

// Build returns the completed interaction.
func (i *Interaction) Build() contract.Interaction {
	return i.interaction
}

// HTTPRequestBuilder builds an HTTP request.
type HTTPRequestBuilder struct {
	interaction *Interaction
}

// WithQuery adds query parameters.
func (b *HTTPRequestBuilder) WithQuery(params map[string]string) *HTTPRequestBuilder {
	b.interaction.interaction.Payload.HTTP.Request.Query = params
	return b
}

// WithHeader adds a header.
func (b *HTTPRequestBuilder) WithHeader(key, value string) *HTTPRequestBuilder {
	if b.interaction.interaction.Payload.HTTP.Request.Headers == nil {
		b.interaction.interaction.Payload.HTTP.Request.Headers = make(map[string]string)
	}
	b.interaction.interaction.Payload.HTTP.Request.Headers[key] = value
	return b
}

// WithJSONBody sets the request body.
func (b *HTTPRequestBuilder) WithJSONBody(body any) *HTTPRequestBuilder {
	b.interaction.interaction.Payload.HTTP.Request.Body = body
	if b.interaction.interaction.Payload.HTTP.Request.Headers == nil {
		b.interaction.interaction.Payload.HTTP.Request.Headers = make(map[string]string)
	}
	b.interaction.interaction.Payload.HTTP.Request.Headers["Content-Type"] = "application/json"
	return b
}

// WillRespondWith starts building the response.
func (b *HTTPRequestBuilder) WillRespondWith(status int) *HTTPResponseBuilder {
	b.interaction.interaction.Payload.HTTP.Response = contract.HTTPResponse{
		Status: status,
	}
	return &HTTPResponseBuilder{interaction: b.interaction}
}

// HTTPResponseBuilder builds an HTTP response.
type HTTPResponseBuilder struct {
	interaction *Interaction
}

// WithHeader adds a response header.
func (b *HTTPResponseBuilder) WithHeader(key, value string) *HTTPResponseBuilder {
	if b.interaction.interaction.Payload.HTTP.Response.Headers == nil {
		b.interaction.interaction.Payload.HTTP.Response.Headers = make(map[string]string)
	}
	b.interaction.interaction.Payload.HTTP.Response.Headers[key] = value
	return b
}

// WithJSONBody sets the response body.
func (b *HTTPResponseBuilder) WithJSONBody(body any) *HTTPResponseBuilder {
	b.interaction.interaction.Payload.HTTP.Response.Body = body
	if b.interaction.interaction.Payload.HTTP.Response.Headers == nil {
		b.interaction.interaction.Payload.HTTP.Response.Headers = make(map[string]string)
	}
	b.interaction.interaction.Payload.HTTP.Response.Headers["Content-Type"] = "application/json"
	return b
}

// Build returns the completed interaction.
func (b *HTTPResponseBuilder) Build() *Interaction {
	return b.interaction
}

// Matching rule helpers

// TypeMatcher creates a type matching rule.
func TypeMatcher() contract.MatchingRule {
	return contract.MatchingRule{Match: contract.MatchType_}
}

// RegexMatcher creates a regex matching rule.
func RegexMatcher(pattern string) contract.MatchingRule {
	return contract.MatchingRule{Match: contract.MatchRegex, Pattern: pattern}
}

// IntegerMatcher creates an integer matching rule.
func IntegerMatcher() contract.MatchingRule {
	return contract.MatchingRule{Match: contract.MatchInteger}
}

// DecimalMatcher creates a decimal matching rule.
func DecimalMatcher() contract.MatchingRule {
	return contract.MatchingRule{Match: contract.MatchDecimal}
}

// IncludeMatcher creates an include matching rule.
func IncludeMatcher(substring string) contract.MatchingRule {
	return contract.MatchingRule{Match: contract.MatchInclude, Value: substring}
}

// EachLikeMatcher creates an each_like matching rule.
func EachLikeMatcher(min int) contract.MatchingRule {
	return contract.MatchingRule{Match: contract.MatchEachLike, Min: &min}
}

// OptionalMatcher creates an optional matching rule.
func OptionalMatcher() contract.MatchingRule {
	return contract.MatchingRule{Match: contract.MatchOptional}
}
