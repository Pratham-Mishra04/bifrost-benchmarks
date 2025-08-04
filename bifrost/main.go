package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost-gateway/lib"
	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

var (
	openaiKey string
	port      string
	proxyURL  string
	debug     bool

	concurrency     int
	bufferSize      int
	initialPoolSize int
)

func init() {
	flag.StringVar(&openaiKey, "openai-key", "", "OpenAI API key")
	flag.StringVar(&port, "port", "3001", "Port to run the server on")
	flag.StringVar(&proxyURL, "proxy", "", "Proxy URL (e.g., http://localhost:8080)")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")

	flag.IntVar(&concurrency, "concurrency", 5000, "Concurrency level")
	flag.IntVar(&bufferSize, "buffer-size", 5000, "Buffer size")
	flag.IntVar(&initialPoolSize, "initial-pool-size", 5000, "Initial pool size")

	flag.Parse()

	if openaiKey == "" {
		file, err := os.Open("../.env")
		if err != nil {
			log.Fatalf("Error loading .env file: %v", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && parts[0] == "OPENAI_API_KEY" {
				openaiKey = parts[1]
				break
			}
		}

		if err := scanner.Err(); err != nil {
			log.Fatalf("Error reading .env file: %v", err)
		}

		if openaiKey == "" {
			log.Fatalf("OpenAI API key is required")
		}
	}
}

type ChatRequest struct {
	Messages []schemas.BifrostMessage `json:"messages"`
	Model    string                   `json:"model"`
}

func main() {
	// Set GOMAXPROCS to utilize all available CPU cores
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Initialize the Bifrost client with connection pooling
	account := lib.NewBaseAccount(openaiKey, proxyURL, concurrency, bufferSize)
	client, err := bifrost.Init(schemas.BifrostConfig{
		Account:         account,
		Plugins:         []schemas.Plugin{},
		Logger:          nil,
		InitialPoolSize: initialPoolSize,
	})
	if err != nil {
		log.Fatalf("Failed to initialize Bifrost: %v", err)
	}

	r := router.New()

	if debug {
		r.POST("/v1/chat/completions", lib.DebugHandler(client))
		r.GET("/metrics", lib.GetMetricsHandler())
	} else {
		Handler := func(ctx *fasthttp.RequestCtx) {
			var chatReq ChatRequest
			if err := json.Unmarshal(ctx.PostBody(), &chatReq); err != nil {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetBodyString(fmt.Sprintf("invalid request format: %v", err))
				return
			}

			if strings.Contains(chatReq.Model, "/") {
				parts := strings.Split(chatReq.Model, "/")
				chatReq.Model = parts[1]
			}

			bifrostReq := &schemas.BifrostRequest{
				Provider: schemas.OpenAI,
				Model:    chatReq.Model,
				Input: schemas.RequestInput{
					ChatCompletionInput: &chatReq.Messages,
				},
			}

			if len(chatReq.Messages) == 0 {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetBodyString("Messages array is required")
				return
			}

			resp, err := client.ChatCompletionRequest(ctx, bifrostReq)
			if err != nil {
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				ctx.SetBodyString(fmt.Sprintf("error: %v", err))
				return
			}

			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetContentType("application/json")
			json.NewEncoder(ctx).Encode(resp)
		}

		// Define HTTP handlers
		r.POST("/v1/chat/completions", Handler)
	}

	// Configure server for high throughput
	server := &fasthttp.Server{
		Handler:               r.Handler,
		NoDefaultServerHeader: true,
		TCPKeepalive:          true,
		Concurrency:           0, // unlimited concurrent connections
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		fmt.Printf("Bifrost API server starting on port %s...\n", port)
		if err := server.ListenAndServe(":" + port); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down server...")

	client.Cleanup()

	// Shutdown server gracefully
	if err := server.Shutdown(); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	if debug {
		// Print statistics
		lib.PrintStats()
	}
}
