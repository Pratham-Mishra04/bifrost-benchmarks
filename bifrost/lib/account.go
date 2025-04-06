package lib

import (
	"fmt"

	"github.com/maximhq/bifrost/interfaces"
)

// BaseAccount provides a basic implementation of the Account interface for Anthropic and OpenAI providers
type BaseAccount struct{}

// GetInitiallyConfiguredProviderKeys returns all provider keys
func (baseAccount *BaseAccount) GetInitiallyConfiguredProviders() ([]interfaces.SupportedModelProvider, error) {
	return []interfaces.SupportedModelProvider{interfaces.OpenAI}, nil
}

// GetConcurrencyAndBufferSizeForProvider returns the concurrency and buffer size settings for a provider
func (baseAccount *BaseAccount) GetConfigForProvider(providerKey interfaces.SupportedModelProvider) (*interfaces.ProviderConfig, error) {
	switch providerKey {
	case interfaces.OpenAI:
		return &interfaces.ProviderConfig{
			NetworkConfig: interfaces.NetworkConfig{
				DefaultRequestTimeoutInSeconds: 30,
			},
			ConcurrencyAndBufferSize: interfaces.ConcurrencyAndBufferSize{
				Concurrency: 5000,
				BufferSize:  7500,
			},
			ProxyConfig: &interfaces.ProxyConfig{
				Type: interfaces.HttpProxy,
				URL:  "http://localhost:8080",
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerKey)
	}
}
