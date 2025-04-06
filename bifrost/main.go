package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost"
	"github.com/maximhq/bifrost/interfaces"
	"github.com/valyala/fasthttp"
)

var (
	openaiKey string
	port      string
	proxyURL  string
)

func init() {
	flag.StringVar(&openaiKey, "openai-key", "", "OpenAI API key")
	flag.StringVar(&port, "port", "3001", "Port to run the server on")
	flag.StringVar(&proxyURL, "proxy", "", "Proxy URL (e.g., http://localhost:8080)")
	flag.Parse()

	if openaiKey == "" {
		log.Fatal("OpenAI API key is required")
	}
}

type ChatRequest struct {
	Messages []interfaces.Message `json:"messages"`
	Model    string               `json:"model"`
}

// CustomAccount implements the Account interface
type BaseAccount struct {
	apiKey string
}

func (a *BaseAccount) GetKeysForProvider(providerKey interfaces.SupportedModelProvider) ([]interfaces.Key, error) {
	if providerKey == interfaces.OpenAI {
		return []interfaces.Key{
			{
				Value:  a.apiKey,
				Models: []string{"gpt-4o-mini", "gpt-4o", "gpt-4-turbo"},
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
			},
			ConcurrencyAndBufferSize: interfaces.ConcurrencyAndBufferSize{
				Concurrency: 5000,
				BufferSize:  7500,
			},
		}

		// Only set proxy configuration if proxy flag is provided
		if proxyURL != "" {
			config.ProxyConfig = &interfaces.ProxyConfig{
				Type: interfaces.HttpProxy,
				URL:  proxyURL,
			}
		}

		return config, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerKey)
	}
}

func main() {
	// Set GOMAXPROCS to utilize all available CPU cores
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Initialize the Bifrost client with connection pooling
	account := &BaseAccount{
		apiKey: openaiKey,
	}
	client, err := bifrost.Init(account, nil, nil)
	if err != nil {
		log.Fatalf("Failed to initialize Bifrost: %v", err)
	}

	r := router.New()

	// Define HTTP handlers
	r.POST("/v1/chat/completions", func(ctx *fasthttp.RequestCtx) {
		// Time request parsing
		var chatReq ChatRequest
		if err := json.Unmarshal(ctx.PostBody(), &chatReq); err != nil {
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			ctx.SetBodyString(fmt.Sprintf("Invalid request format: %v", err))
			return
		}

		if len(chatReq.Messages) == 0 {
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			ctx.SetBodyString("Messages array is required")
			return
		}

		// Create Bifrost request
		bifrostReq := &interfaces.BifrostRequest{
			Model: chatReq.Model,
			Input: interfaces.RequestInput{
				ChatCompletionInput: &chatReq.Messages,
			},
		}

		// Time Bifrost API call
		bifrostResp, bifrostErr := client.ChatCompletionRequest(interfaces.OpenAI, bifrostReq, nil)

		if bifrostErr != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBodyString(fmt.Sprintf("Error processing request: %v", bifrostErr.Error.Message))
			return
		}

		// Send response
		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(bifrostResp)
	})

	// Configure server for high throughput
	server := &fasthttp.Server{
		Handler:               r.Handler,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
		IdleTimeout:           30 * time.Second,
		MaxRequestsPerConn:    5000,
		MaxConnsPerIP:         10000,
		MaxRequestBodySize:    10 * 1024 * 1024, // 10MB
		NoDefaultServerHeader: true,
		ReduceMemoryUsage:     true,
		GetOnly:               false,
		DisableKeepalive:      false,
		TCPKeepalive:          true,
		TCPKeepalivePeriod:    30 * time.Second,
		NoDefaultContentType:  true,
		MaxIdleWorkerDuration: 10 * time.Second,
		ReadBufferSize:        4096,
		WriteBufferSize:       4096,
	}

	// Start server
	fmt.Printf("Bifrost API server starting on port %s...\n", port)
	log.Fatal(server.ListenAndServe(":" + port))
}
