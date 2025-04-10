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
	"github.com/maximhq/bifrost"
	"github.com/maximhq/bifrost-gateway/lib"
	"github.com/maximhq/bifrost/interfaces"
	"github.com/valyala/fasthttp"
)

var (
	openaiKey string
	port      string
	proxyURL  string
	debug     bool
)

func init() {
	flag.StringVar(&openaiKey, "openai-key", "", "OpenAI API key")
	flag.StringVar(&port, "port", "3001", "Port to run the server on")
	flag.StringVar(&proxyURL, "proxy", "", "Proxy URL (e.g., http://localhost:8080)")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
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
	Messages []interfaces.Message `json:"messages"`
	Model    string               `json:"model"`
}

func main() {
	// Set GOMAXPROCS to utilize all available CPU cores
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Initialize the Bifrost client with connection pooling
	account := lib.NewBaseAccount(openaiKey)
	client, err := bifrost.Init(interfaces.BifrostConfig{
		Account:         account,
		Plugins:         []interfaces.Plugin{},
		Logger:          nil,
		InitialPoolSize: 8000,
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

			bifrostReq := &interfaces.BifrostRequest{
				Model: chatReq.Model,
				Input: interfaces.RequestInput{
					ChatCompletionInput: &chatReq.Messages,
				},
			}

			if len(chatReq.Messages) == 0 {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetBodyString("Messages array is required")
				return
			}

			resp, err := client.ChatCompletionRequest(interfaces.OpenAI, bifrostReq, ctx)
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
		ReduceMemoryUsage:     false,
		DisableKeepalive:      false,
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

	client.Shutdown()

	// Shutdown server gracefully
	if err := server.Shutdown(); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	if debug {
		// Print statistics
		lib.PrintStats()
	}
}
