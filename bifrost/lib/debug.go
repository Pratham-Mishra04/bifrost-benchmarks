package lib

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

// RequestMetrics holds timing metrics from Bifrost
type RequestMetrics struct {
	QueueWaitTime    time.Duration `json:"queue_wait_time"`
	KeySelectionTime time.Duration `json:"key_selection_time"`
	PluginPreTime    time.Duration `json:"plugin_pre_time"`
	PluginPostTime   time.Duration `json:"plugin_post_time"`
	RequestCount     int64         `json:"request_count"`
	ErrorCount       int64         `json:"error_count"`
}

// ProviderMetrics holds provider-specific timing metrics
type ProviderMetrics struct {
	MessageFormatting      time.Duration `json:"message_formatting"`
	ParamsPreparation      time.Duration `json:"params_preparation"`
	RequestBodyPreparation time.Duration `json:"request_body_preparation"`
	JSONMarshaling         time.Duration `json:"json_marshaling"`
	RequestSetup           time.Duration `json:"request_setup"`
	HTTPRequest            time.Duration `json:"http_request"`
	ErrorHandling          time.Duration `json:"error_handling"`
	ResponseParsing        time.Duration `json:"response_parsing"`
	RequestSizeInBytes     int64         `json:"request_size_in_bytes"`
	ResponseSizeInBytes    int64         `json:"response_size_in_bytes"`
}

// TimingStats holds timing statistics
type TimingStats struct {
	mu              sync.Mutex
	totalRequests   int
	metrics         []RequestMetrics
	timings         []time.Duration
	providerMetrics []ProviderMetrics
}

// ServerMetrics tracks server-level metrics
type ServerMetrics struct {
	mu                 sync.Mutex
	TotalRequests      int64
	SuccessfulRequests int64
	DroppedRequests    int64
	QueueSize          int64
	ErrorCount         int64
	LastError          error
	LastErrorTime      time.Time
}

var (
	stats         = &TimingStats{}
	serverMetrics = &ServerMetrics{}
)

func formatSmartDuration(ns int64) string {
	avg := float64(ns)
	switch {
	case avg >= 1e9:
		return fmt.Sprintf("%.2f s", avg/1e9)
	case avg >= 1e6:
		return fmt.Sprintf("%.2f ms", avg/1e6)
	case avg >= 1e3:
		return fmt.Sprintf("%.2f µs", avg/1e3)
	default:
		return fmt.Sprintf("%d ns", ns)
	}
}

func PrintStats() {
	stats.mu.Lock()
	defer stats.mu.Unlock()

	if stats.totalRequests == 0 {
		fmt.Println("No requests processed")
		return
	}

	// Calculate averages for Bifrost metrics
	var totalMetrics RequestMetrics
	for _, m := range stats.metrics {
		totalMetrics.QueueWaitTime += m.QueueWaitTime
		totalMetrics.KeySelectionTime += m.KeySelectionTime
		totalMetrics.PluginPreTime += m.PluginPreTime
		totalMetrics.PluginPostTime += m.PluginPostTime
		totalMetrics.RequestCount += m.RequestCount
		totalMetrics.ErrorCount += m.ErrorCount
	}

	// Calculate averages for provider timings
	var totalProviderMetrics ProviderMetrics
	for _, t := range stats.providerMetrics {
		totalProviderMetrics.MessageFormatting += t.MessageFormatting
		totalProviderMetrics.ParamsPreparation += t.ParamsPreparation
		totalProviderMetrics.RequestBodyPreparation += t.RequestBodyPreparation
		totalProviderMetrics.JSONMarshaling += t.JSONMarshaling
		totalProviderMetrics.RequestSetup += t.RequestSetup
		totalProviderMetrics.HTTPRequest += t.HTTPRequest
		totalProviderMetrics.ErrorHandling += t.ErrorHandling
		totalProviderMetrics.ResponseParsing += t.ResponseParsing
		totalProviderMetrics.RequestSizeInBytes += t.RequestSizeInBytes
		totalProviderMetrics.ResponseSizeInBytes += t.ResponseSizeInBytes
	}

	// Calculate averages for timings
	var totalTimings time.Duration
	for _, t := range stats.timings {
		totalTimings += t
	}

	// Print final metrics
	serverMetrics.mu.Lock()
	fmt.Printf("\nServer Metrics:\n")
	fmt.Printf("Total Requests: %d\n", serverMetrics.TotalRequests)
	fmt.Printf("Successful Requests: %d\n", serverMetrics.SuccessfulRequests)
	fmt.Printf("Dropped Requests: %d\n", serverMetrics.DroppedRequests)
	fmt.Printf("Error Count: %d\n", serverMetrics.ErrorCount)
	fmt.Printf("Last Error: %s\n", serverMetrics.LastError)
	fmt.Printf("Last Error Time: %v\n", serverMetrics.LastErrorTime)
	serverMetrics.mu.Unlock()

	fmt.Printf("\nTiming Statistics:\n")
	fmt.Printf("Total Requests: %d\n", stats.totalRequests)

	fmt.Printf("\nBifrost Metrics (averages):\n")
	// Check if we have provider timings to avoid division by zero
	if len(stats.providerMetrics) > 0 {
		fmt.Printf("Queue Wait Time: %s\n", formatSmartDuration(totalMetrics.QueueWaitTime.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("Key Selection Time: %s\n", formatSmartDuration(totalMetrics.KeySelectionTime.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("Plugin Pre Time: %s\n", formatSmartDuration(totalMetrics.PluginPreTime.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("Plugin Post Time: %s\n", formatSmartDuration(totalMetrics.PluginPostTime.Nanoseconds()/int64(len(stats.providerMetrics))))

		fmt.Printf("\nProvider Timings (averages):\n")
		fmt.Printf("Message Formatting: %s\n", formatSmartDuration(totalProviderMetrics.MessageFormatting.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("Params Preparation: %s\n", formatSmartDuration(totalProviderMetrics.ParamsPreparation.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("Request Body Preparation: %s\n", formatSmartDuration(totalProviderMetrics.RequestBodyPreparation.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("JSON Marshaling: %s\n", formatSmartDuration(totalProviderMetrics.JSONMarshaling.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("Request Setup: %s\n", formatSmartDuration(totalProviderMetrics.RequestSetup.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("HTTP Request: %s\n", formatSmartDuration(totalProviderMetrics.HTTPRequest.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("Error Handling: %s\n", formatSmartDuration(totalProviderMetrics.ErrorHandling.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("Response Parsing: %s\n", formatSmartDuration(totalProviderMetrics.ResponseParsing.Nanoseconds()/int64(len(stats.providerMetrics))))
		fmt.Printf("Request Size: %.2f KB\n", float64(totalProviderMetrics.RequestSizeInBytes)/float64(len(stats.providerMetrics))/1024.0)
		fmt.Printf("Response Size: %.2f KB\n", float64(totalProviderMetrics.ResponseSizeInBytes)/float64(len(stats.providerMetrics))/1024.0)
	} else {
		fmt.Println("No provider timing data available")
	}

	// Only calculate average timings if we have data
	if len(stats.timings) > 0 {
		avgTimings := float64(totalTimings) / float64(len(stats.timings)) / float64(time.Nanosecond)
		fmt.Printf("\nAverage Timings: %.2f ms\n", avgTimings)
	}
}

type ChatRequest struct {
	Messages []schemas.Message `json:"messages"`
	Model    string            `json:"model"`
}

func DebugHandler(client *bifrost.Bifrost) func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		// Track incoming request
		serverMetrics.mu.Lock()
		serverMetrics.TotalRequests++
		serverMetrics.mu.Unlock()

		// Time request parsing
		var chatReq ChatRequest
		if err := json.Unmarshal(ctx.PostBody(), &chatReq); err != nil {
			serverMetrics.mu.Lock()
			serverMetrics.ErrorCount++
			serverMetrics.LastError = fmt.Errorf("invalid request format: %v", err)
			serverMetrics.LastErrorTime = time.Now()
			serverMetrics.mu.Unlock()

			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			ctx.SetBodyString(fmt.Sprintf("invalid request format: %v", err))
			return
		}

		if len(chatReq.Messages) == 0 {
			serverMetrics.mu.Lock()
			serverMetrics.ErrorCount++
			serverMetrics.LastError = fmt.Errorf("messages array is required")
			serverMetrics.LastErrorTime = time.Now()
			serverMetrics.mu.Unlock()

			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			ctx.SetBodyString("Messages array is required")
			return
		}

		// Create Bifrost request
		bifrostReq := &schemas.BifrostRequest{
			Provider: schemas.OpenAI,
			Model:    chatReq.Model,
			Input: schemas.RequestInput{
				ChatCompletionInput: &chatReq.Messages,
			},
		}

		// Make Bifrost API call with timeout
		done := make(chan struct{})
		var bifrostResp *schemas.BifrostResponse
		var bifrostErr *schemas.BifrostError

		go func() {
			bifrostResp, bifrostErr = client.ChatCompletionRequest(ctx, bifrostReq)
			close(done)
		}()

		select {
		case <-done:
			// Request completed
		case <-time.After(30 * time.Second):
			// Request timed out
			serverMetrics.mu.Lock()
			serverMetrics.DroppedRequests++
			serverMetrics.LastError = fmt.Errorf("request timed out after 30 seconds")
			serverMetrics.LastErrorTime = time.Now()
			serverMetrics.mu.Unlock()

			ctx.SetStatusCode(fasthttp.StatusGatewayTimeout)
			ctx.SetBodyString("Request timed out")
			return
		}

		if bifrostErr != nil {
			serverMetrics.mu.Lock()
			serverMetrics.ErrorCount++
			serverMetrics.LastError = bifrostErr.Error.Error
			serverMetrics.LastErrorTime = time.Now()
			serverMetrics.mu.Unlock()

			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetContentType("application/json")
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			json.NewEncoder(ctx).Encode(bifrostErr)
			return
		}

		// Track successful request
		serverMetrics.mu.Lock()
		serverMetrics.SuccessfulRequests++
		serverMetrics.mu.Unlock()

		// Extract timing information from response
		stats.mu.Lock()
		stats.totalRequests++

		if rawResponse, ok := bifrostResp.ExtraFields.RawResponse.(map[string]interface{}); ok {
			// Process bifrost_timings
			if metrics, ok := rawResponse["bifrost_timings"]; ok {
				// Convert to JSON bytes first
				jsonBytes, err := json.Marshal(metrics)
				if err != nil {
					fmt.Printf("Error marshaling bifrost_timings: %v\n", err)
					return
				}
				// Unmarshal into RequestMetrics
				var requestMetrics RequestMetrics
				if err := json.Unmarshal(jsonBytes, &requestMetrics); err != nil {
					fmt.Printf("Error unmarshaling bifrost_timings: %v\n", err)
					return
				}
				stats.metrics = append(stats.metrics, requestMetrics)
			}

			// Process provider_metrics
			if metrics, ok := rawResponse["provider_metrics"]; ok {
				// Convert to JSON bytes first
				jsonBytes, err := json.Marshal(metrics)
				if err != nil {
					fmt.Printf("Error marshaling provider_metrics: %v\n", err)
					return
				}

				// Unmarshal into ProviderMetrics
				var providerMetrics ProviderMetrics
				if err := json.Unmarshal(jsonBytes, &providerMetrics); err != nil {
					fmt.Printf("Error unmarshaling provider_metrics: %v\n", err)
					return
				}

				stats.providerMetrics = append(stats.providerMetrics, providerMetrics)
			}
		}

		stats.mu.Unlock()

		// Send response
		ctx.SetContentType("application/json")

		// Add recovery to prevent panics during JSON encoding
		defer func() {
			if r := recover(); r != nil {
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				ctx.SetBodyString(fmt.Sprintf("Error encoding response: %v", r))
				log.Printf("Panic during response encoding: %v", r)
			}
		}()

		// Additional safety check before encoding
		if bifrostResp == nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBodyString("Error: nil response from Bifrost")
			return
		}

		// Encode the response
		if err := json.NewEncoder(ctx).Encode(bifrostResp); err != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBodyString(fmt.Sprintf("Error encoding response: %v", err))
			log.Printf("Error encoding response: %v", err)
		}
	}
}

func GetMetricsHandler() func(ctx *fasthttp.RequestCtx) {
	return func(ctx *fasthttp.RequestCtx) {
		serverMetrics.mu.Lock()
		defer serverMetrics.mu.Unlock()

		metrics := map[string]interface{}{
			"total_requests":      serverMetrics.TotalRequests,
			"successful_requests": serverMetrics.SuccessfulRequests,
			"dropped_requests":    serverMetrics.DroppedRequests,
			"error_count":         serverMetrics.ErrorCount,
			"last_error":          serverMetrics.LastError,
			"last_error_time":     serverMetrics.LastErrorTime,
			"current_time":        time.Now(),
		}

		ctx.SetContentType("application/json")
		json.NewEncoder(ctx).Encode(metrics)
	}
}
