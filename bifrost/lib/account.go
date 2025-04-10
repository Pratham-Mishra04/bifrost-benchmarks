package lib

import (
	"fmt"
	"time"

	"github.com/maximhq/bifrost/interfaces"
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

func (a *BaseAccount) GetKeysForProvider(providerKey interfaces.SupportedModelProvider) ([]interfaces.Key, error) {
	if providerKey == interfaces.OpenAI {
		return []interfaces.Key{
			{
				Value:  a.apiKey,
				Models: []string{"gpt-4o-mini", "gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo"},
				Weight: 1.0,
			},
		}, nil
	}

	return nil, fmt.Errorf("unsupported provider: %s", providerKey)
}

func (baseAccount *BaseAccount) GetInitiallyConfiguredProviders() ([]interfaces.SupportedModelProvider, error) {
	return []interfaces.SupportedModelProvider{interfaces.OpenAI}, nil
}

// GetConcurrencyAndBufferSizeForProvider returns the concurrency and buffer size settings for a provider
func (baseAccount *BaseAccount) GetConfigForProvider(providerKey interfaces.SupportedModelProvider) (*interfaces.ProviderConfig, error) {
	switch providerKey {
	case interfaces.OpenAI:
		config := &interfaces.ProviderConfig{
			NetworkConfig: interfaces.NetworkConfig{
				DefaultRequestTimeoutInSeconds: 30,
				MaxRetries:                     3,
				RetryBackoffInitial:            100 * time.Millisecond,
				RetryBackoffMax:                2 * time.Second,
			},
			ConcurrencyAndBufferSize: interfaces.ConcurrencyAndBufferSize{
				Concurrency: 8000,
				BufferSize:  10000,
			},
		}

		// Only set proxy configuration if proxy flag is provided
		if baseAccount.proxyURL != "" {
			config.ProxyConfig = &interfaces.ProxyConfig{
				Type: interfaces.HttpProxy,
				URL:  baseAccount.proxyURL,
			}
		}

		return config, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerKey)
	}
}
