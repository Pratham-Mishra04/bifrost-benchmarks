package lib

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/maximhq/bifrost"
	"github.com/maximhq/bifrost/interfaces"
	"github.com/valyala/fasthttp"
)

// RequestMetrics holds timing metrics from Bifrost
type RequestMetrics struct {
	TotalTime        time.Duration `json:"total_time"`
	QueueWaitTime    time.Duration `json:"queue_wait_time"`
	KeySelectionTime time.Duration `json:"key_selection_time"`
	ProviderTime     time.Duration `json:"provider_time"`
	PluginPreTime    time.Duration `json:"plugin_pre_time"`
	PluginPostTime   time.Duration `json:"plugin_post_time"`
	RequestCount     int64         `json:"request_count"`
	ErrorCount       int64         `json:"error_count"`
}

// ProviderTimings holds provider-specific timing metrics
type ProviderTimings struct {
	MessageFormatting      time.Duration `json:"message_formatting"`
	ParamsPreparation      time.Duration `json:"params_preparation"`
	RequestBodyPreparation time.Duration `json:"request_body_preparation"`
	JSONMarshaling         time.Duration `json:"json_marshaling"`
	RequestSetup           time.Duration `json:"request_setup"`
	HTTPRequest            time.Duration `json:"http_request"`
	ErrorHandling          time.Duration `json:"error_handling"`
	ResponseParsing        time.Duration `json:"response_parsing"`
}

// TimingStats holds timing statistics
type TimingStats struct {
	mu              sync.Mutex
	totalRequests   int
	metrics         []RequestMetrics
	timings         []time.Duration
	providerTimings []ProviderTimings
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

// ExtractMetrics extracts timing information from Bifrost response
func ExtractMetrics(bifrostResp *interfaces.BifrostResponse) {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	stats.totalRequests++

	if rawResponse, ok := bifrostResp.ExtraFields.RawResponse.(map[string]interface{}); ok {
		var requestMetrics RequestMetrics
		var providerTimings ProviderTimings

		// Process bifrost_timings
		if metrics, ok := rawResponse["bifrost_timings"]; ok {
			if metricsMap, ok := metrics.(map[string]interface{}); ok {
				if v, ok := metricsMap["total_time"].(float64); ok {
					requestMetrics.TotalTime = time.Duration(v)
				}
				if v, ok := metricsMap["queue_wait_time"].(float64); ok {
					requestMetrics.QueueWaitTime = time.Duration(v)
				}
				if v, ok := metricsMap["key_selection_time"].(float64); ok {
					requestMetrics.KeySelectionTime = time.Duration(v)
				}
				if v, ok := metricsMap["provider_time"].(float64); ok {
					requestMetrics.ProviderTime = time.Duration(v)
				}
				if v, ok := metricsMap["plugin_pre_time"].(float64); ok {
					requestMetrics.PluginPreTime = time.Duration(v)
				}
				if v, ok := metricsMap["plugin_post_time"].(float64); ok {
					requestMetrics.PluginPostTime = time.Duration(v)
				}
				stats.metrics = append(stats.metrics, requestMetrics)
			}
		}

		// Process timings
		if timings, ok := rawResponse["timings"]; ok {
			if timingsMap, ok := timings.(map[string]interface{}); ok {
				if v, ok := timingsMap["message_formatting"].(float64); ok {
					providerTimings.MessageFormatting = time.Duration(v)
				}
				if v, ok := timingsMap["params_preparation"].(float64); ok {
					providerTimings.ParamsPreparation = time.Duration(v)
				}
				if v, ok := timingsMap["request_body_preparation"].(float64); ok {
					providerTimings.RequestBodyPreparation = time.Duration(v)
				}
				if v, ok := timingsMap["json_marshaling"].(float64); ok {
					providerTimings.JSONMarshaling = time.Duration(v)
				}
				if v, ok := timingsMap["request_setup"].(float64); ok {
					providerTimings.RequestSetup = time.Duration(v)
				}
				if v, ok := timingsMap["http_request"].(float64); ok {
					providerTimings.HTTPRequest = time.Duration(v)
				}
				if v, ok := timingsMap["error_handling"].(float64); ok {
					providerTimings.ErrorHandling = time.Duration(v)
				}
				if v, ok := timingsMap["response_parsing"].(float64); ok {
					providerTimings.ResponseParsing = time.Duration(v)
				}
				stats.providerTimings = append(stats.providerTimings, providerTimings)
			}
		}
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
		totalMetrics.TotalTime += m.TotalTime
		totalMetrics.QueueWaitTime += m.QueueWaitTime
		totalMetrics.KeySelectionTime += m.KeySelectionTime
		totalMetrics.ProviderTime += m.ProviderTime
		totalMetrics.PluginPreTime += m.PluginPreTime
		totalMetrics.PluginPostTime += m.PluginPostTime
		totalMetrics.RequestCount += m.RequestCount
		totalMetrics.ErrorCount += m.ErrorCount
	}

	// Calculate averages for provider timings
	var totalProviderTimings ProviderTimings
	for _, t := range stats.providerTimings {
		totalProviderTimings.MessageFormatting += t.MessageFormatting
		totalProviderTimings.ParamsPreparation += t.ParamsPreparation
		totalProviderTimings.RequestBodyPreparation += t.RequestBodyPreparation
		totalProviderTimings.JSONMarshaling += t.JSONMarshaling
		totalProviderTimings.RequestSetup += t.RequestSetup
		totalProviderTimings.HTTPRequest += t.HTTPRequest
		totalProviderTimings.ErrorHandling += t.ErrorHandling
		totalProviderTimings.ResponseParsing += t.ResponseParsing
	}

	// Calculate averages for timings
	var totalTimings time.Duration
	for _, t := range stats.timings {
		totalTimings += t
	}

	avgTimings := float64(totalTimings) / float64(len(stats.timings)) / float64(time.Millisecond)

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
	fmt.Printf("Total Time: %.2f ms\n", float64(totalMetrics.TotalTime)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("Queue Wait Time: %.2f ms\n", float64(totalMetrics.QueueWaitTime)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("Key Selection Time: %.2f ms\n", float64(totalMetrics.KeySelectionTime)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("Provider Time: %.2f ms\n", float64(totalMetrics.ProviderTime)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("Plugin Pre Time: %.2f ms\n", float64(totalMetrics.PluginPreTime)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("Plugin Post Time: %.2f ms\n", float64(totalMetrics.PluginPostTime)/float64(stats.totalRequests)/float64(time.Millisecond))

	fmt.Printf("\nProvider Timings (averages):\n")
	fmt.Printf("Message Formatting: %.2f ms\n", float64(totalProviderTimings.MessageFormatting)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("Params Preparation: %.2f ms\n", float64(totalProviderTimings.ParamsPreparation)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("Request Body Preparation: %.2f ms\n", float64(totalProviderTimings.RequestBodyPreparation)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("JSON Marshaling: %.2f ms\n", float64(totalProviderTimings.JSONMarshaling)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("Request Setup: %.2f ms\n", float64(totalProviderTimings.RequestSetup)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("HTTP Request: %.2f ms\n", float64(totalProviderTimings.HTTPRequest)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("Error Handling: %.2f ms\n", float64(totalProviderTimings.ErrorHandling)/float64(stats.totalRequests)/float64(time.Millisecond))
	fmt.Printf("Response Parsing: %.2f ms\n", float64(totalProviderTimings.ResponseParsing)/float64(stats.totalRequests)/float64(time.Millisecond))

	fmt.Printf("\nAverage Timings: %.2f ms\n", avgTimings)
}

type ChatRequest struct {
	Messages []interfaces.Message `json:"messages"`
	Model    string               `json:"model"`
}

// PrintServerMetrics prints server-level metrics
func PrintServerMetrics() {
	serverMetrics.mu.Lock()
	defer serverMetrics.mu.Unlock()

	fmt.Printf("\nServer Metrics:\n")
	fmt.Printf("Total Requests: %d\n", serverMetrics.TotalRequests)
	fmt.Printf("Successful Requests: %d\n", serverMetrics.SuccessfulRequests)
	fmt.Printf("Dropped Requests: %d\n", serverMetrics.DroppedRequests)
	fmt.Printf("Error Count: %d\n", serverMetrics.ErrorCount)
	fmt.Printf("Last Error: %v\n", serverMetrics.LastError)
	fmt.Printf("Last Error Time: %v\n", serverMetrics.LastErrorTime)
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
		bifrostReq := &interfaces.BifrostRequest{
			Model: chatReq.Model,
			Input: interfaces.RequestInput{
				ChatCompletionInput: &chatReq.Messages,
			},
		}

		// Make Bifrost API call with timeout
		done := make(chan struct{})
		var bifrostResp *interfaces.BifrostResponse
		var bifrostErr *interfaces.BifrostError

		go func() {
			bifrostResp, bifrostErr = client.ChatCompletionRequest(interfaces.OpenAI, bifrostReq, nil)
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
			ctx.SetBodyString(fmt.Sprintf("Error processing request: %v", bifrostErr.Error.Message))
			return
		}

		// Track successful request
		serverMetrics.mu.Lock()
		serverMetrics.SuccessfulRequests++
		serverMetrics.mu.Unlock()

		// Extract timing information from response
		stats.mu.Lock()
		stats.totalRequests++

		// Extract Bifrost metrics more efficiently
		if rawResponse, ok := bifrostResp.ExtraFields.RawResponse.(map[string]interface{}); ok {
			// Pre-allocate metrics
			var requestMetrics RequestMetrics
			var providerTimings ProviderTimings

			// Process bifrost_timings
			if metrics, ok := rawResponse["bifrost_timings"]; ok {
				if metricsMap, ok := metrics.(map[string]interface{}); ok {
					// Direct assignment to avoid marshal/unmarshal
					if v, ok := metricsMap["total_time"].(float64); ok {
						requestMetrics.TotalTime = time.Duration(v)
					}
					if v, ok := metricsMap["queue_wait_time"].(float64); ok {
						requestMetrics.QueueWaitTime = time.Duration(v)
					}
					if v, ok := metricsMap["key_selection_time"].(float64); ok {
						requestMetrics.KeySelectionTime = time.Duration(v)
					}
					if v, ok := metricsMap["provider_time"].(float64); ok {
						requestMetrics.ProviderTime = time.Duration(v)
					}
					if v, ok := metricsMap["plugin_pre_time"].(float64); ok {
						requestMetrics.PluginPreTime = time.Duration(v)
					}
					if v, ok := metricsMap["plugin_post_time"].(float64); ok {
						requestMetrics.PluginPostTime = time.Duration(v)
					}
					stats.metrics = append(stats.metrics, requestMetrics)
				}
			}

			// Process timings
			if timings, ok := rawResponse["timings"]; ok {
				if timingsMap, ok := timings.(map[string]interface{}); ok {
					// Direct assignment to avoid marshal/unmarshal
					if v, ok := timingsMap["message_formatting"].(float64); ok {
						providerTimings.MessageFormatting = time.Duration(v)
					}
					if v, ok := timingsMap["params_preparation"].(float64); ok {
						providerTimings.ParamsPreparation = time.Duration(v)
					}
					if v, ok := timingsMap["request_body_preparation"].(float64); ok {
						providerTimings.RequestBodyPreparation = time.Duration(v)
					}
					if v, ok := timingsMap["json_marshaling"].(float64); ok {
						providerTimings.JSONMarshaling = time.Duration(v)
					}
					if v, ok := timingsMap["request_setup"].(float64); ok {
						providerTimings.RequestSetup = time.Duration(v)
					}
					if v, ok := timingsMap["http_request"].(float64); ok {
						providerTimings.HTTPRequest = time.Duration(v)
					}
					if v, ok := timingsMap["error_handling"].(float64); ok {
						providerTimings.ErrorHandling = time.Duration(v)
					}
					if v, ok := timingsMap["response_parsing"].(float64); ok {
						providerTimings.ResponseParsing = time.Duration(v)
					}
					stats.providerTimings = append(stats.providerTimings, providerTimings)
				}
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
