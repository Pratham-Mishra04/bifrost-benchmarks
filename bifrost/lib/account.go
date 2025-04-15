package lib

import (
	"fmt"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

// CustomAccount implements the Account interface
type BaseAccount struct {
	apiKey   string
	proxyURL string
}

func NewBaseAccount(apiKey string, proxyURL string) *BaseAccount {
	return &BaseAccount{
		apiKey:   apiKey,
		proxyURL: proxyURL,
	}
}

func (a *BaseAccount) GetKeysForProvider(providerKey schemas.ModelProvider) ([]schemas.Key, error) {
	if providerKey == schemas.OpenAI {
		return []schemas.Key{
			{
				Value:  a.apiKey,
				Models: []string{"gpt-4o-mini", "gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo"},
				Weight: 1.0,
			},
		}, nil
	}

	return nil, fmt.Errorf("unsupported provider: %s", providerKey)
}

func (baseAccount *BaseAccount) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return []schemas.ModelProvider{schemas.OpenAI}, nil
}

// GetConcurrencyAndBufferSizeForProvider returns the concurrency and buffer size settings for a provider
func (baseAccount *BaseAccount) GetConfigForProvider(providerKey schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	switch providerKey {
	case schemas.OpenAI:
		config := &schemas.ProviderConfig{
			NetworkConfig: schemas.NetworkConfig{
				DefaultRequestTimeoutInSeconds: 12,
				MaxRetries:                     3,
				RetryBackoffInitial:            100 * time.Millisecond,
				RetryBackoffMax:                5 * time.Second,
			},
			ConcurrencyAndBufferSize: schemas.ConcurrencyAndBufferSize{
				Concurrency: 25000,
				BufferSize:  30000,
			},
		}

		// Only set proxy configuration if proxy flag is provided
		if baseAccount.proxyURL != "" {
			config.ProxyConfig = &schemas.ProxyConfig{
				Type: schemas.HttpProxy,
				URL:  baseAccount.proxyURL,
			}
		}

		return config, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerKey)
	}
}
