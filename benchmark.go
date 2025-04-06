package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/shirou/gopsutil/v3/process"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

// Provider represents an API provider to be benchmarked
type Provider struct {
	Name     string
	Endpoint string
	Port     string
	Payload  []byte
}

// BenchmarkResult holds the metrics from a benchmark run
type BenchmarkResult struct {
	ProviderName      string
	Metrics           *vegeta.Metrics
	CPUUsage          float64
	ServerMemoryStats []ServerMemStat // Added server memory monitoring
}

// MemStat captures memory statistics
type MemStat struct {
	Alloc      uint64
	TotalAlloc uint64
	Sys        uint64
	NumGC      uint32
}

// ServerMemStat captures server memory usage over time
type ServerMemStat struct {
	Timestamp  time.Time
	RSS        uint64  // Resident Set Size in bytes
	VMS        uint64  // Virtual Memory Size in bytes
	MemPercent float64 // Memory usage as percentage
}

func main() {
	// Define command line flags
	rate := flag.Int("rate", 500, "Requests per second")
	duration := flag.Int("duration", 5, "Duration of test in seconds")
	outputFile := flag.String("output", "results.json", "Output file for results")
	cooldown := flag.Int("cooldown", 5, "Cooldown period between tests in seconds")
	provider := flag.String("provider", "", "Specific provider to benchmark (bifrost, portkey, braintrust, llmlite, openrouter)")
	flag.Parse()

	// Initialize providers
	providers := initializeProviders()

	// Filter providers if specific provider is requested
	if *provider != "" {
		filteredProviders := make([]Provider, 0)
		for _, p := range providers {
			if strings.EqualFold(p.Name, *provider) {
				filteredProviders = append(filteredProviders, p)
				break
			}
		}
		if len(filteredProviders) == 0 {
			log.Fatalf("Provider '%s' not found. Available providers: %v", *provider, getProviderNames(providers))
		}
		providers = filteredProviders
	} else {
		fmt.Println("No specific provider specified. Running benchmarks for all providers...")
	}

	// Run benchmarks
	results := runBenchmarks(providers, *rate, *duration, *cooldown)

	// Save results
	saveResults(results, *outputFile)
}

// Helper function to get provider names
func getProviderNames(providers []Provider) []string {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = strings.ToLower(p.Name)
	}
	return names
}

func initializeProviders() []Provider {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Create payload template with dynamic content placeholders
	payload, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{
			{
				"role": "user",
				"content": "This is a benchmark request #{request_index} at #{timestamp}. " +
					"Please provide a short response about the following topic: " +
					"Explain the concept of Proxy Gateway in the context of AI. ",
			},
		},
		"model": "gpt-4o-mini",
	})

	baseUrl := "http://localhost:%s/v1/chat/completions"

	// Create providers with ports from .env
	providers := []Provider{
		{
			Name:     "Bifrost",
			Endpoint: fmt.Sprintf(baseUrl, os.Getenv("BIFROST_PORT")),
			Port:     os.Getenv("BIFROST_PORT"),
			Payload:  payload,
		},
		{
			Name:     "Portkey",
			Endpoint: fmt.Sprintf(baseUrl, os.Getenv("PORTKEY_PORT")),
			Port:     os.Getenv("PORTKEY_PORT"),
			Payload:  payload,
		},
		// {
		// 	Name:     "Braintrust",
		// 	Endpoint: fmt.Sprintf(baseUrl, os.Getenv("BRAINTRUST_PORT")),
		// 	Port:     os.Getenv("BRAINTRUST_PORT"),
		// 	Payload:  payload,
		// },
		// {
		// 	Name:     "LLMLite",
		// 	Endpoint: fmt.Sprintf(baseUrl, os.Getenv("LLMLITE_PORT")),
		// 	Port:     os.Getenv("LLMLITE_PORT"),
		// 	Payload:  payload,
		// },
		// {
		// 	Name:     "OpenRouter",
		// 	Endpoint: fmt.Sprintf(baseUrl, os.Getenv("OPENROUTER_PORT")),
		// 	Port:     os.Getenv("OPENROUTER_PORT"),
		// 	Payload:  payload,
		// },
	}

	return providers
}

func runBenchmarks(providers []Provider, rate int, duration int, cooldown int) []BenchmarkResult {
	results := make([]BenchmarkResult, 0, len(providers))

	for i, provider := range providers {
		fmt.Printf("Benchmarking %s...\n", provider.Name)

		// Define the attack
		targeter := createTargeter(provider)
		attacker := vegeta.NewAttacker()

		// Setup memory monitoring for the server
		var serverMemStats []ServerMemStat
		var memMutex sync.Mutex
		stopMonitoring := make(chan struct{})
		var wg sync.WaitGroup

		// Start server memory monitoring
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := getProcessByPort(provider.Port)
			if err != nil {
				log.Printf("Warning: Could not find process on port %s: %v", provider.Port, err)
				return // Exit goroutine but allow benchmark to continue
			}

			monitorServerMemory(p, stopMonitoring, &serverMemStats, &memMutex)
		}()

		// Create context with timeout for the attack
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(duration+30)*time.Second) // 30s buffer
		defer cancel()

		// Run the benchmark
		var metrics vegeta.Metrics
		attackRate := vegeta.Rate{Freq: rate, Per: time.Second}
		for res := range attacker.Attack(targeter, attackRate, time.Duration(duration)*time.Second, provider.Name) {
			metrics.Add(res)

			// Check if context is done
			select {
			case <-ctx.Done():
				log.Printf("Attack for %s timed out", provider.Name)
				goto EndAttack
			default:
				// Continue with the attack
			}
		}

	EndAttack:
		metrics.Close()

		// Stop memory monitoring
		close(stopMonitoring)
		wg.Wait() // Wait for monitoring goroutine to finish

		// Lock while copying memory stats to ensure thread safety
		memMutex.Lock()
		serverMemStatsCopy := make([]ServerMemStat, len(serverMemStats))
		copy(serverMemStatsCopy, serverMemStats)
		memMutex.Unlock()

		// Add results
		results = append(results, BenchmarkResult{
			ProviderName:      provider.Name,
			Metrics:           &metrics,
			ServerMemoryStats: serverMemStatsCopy,
		})

		fmt.Println(metrics.StatusCodes)

		// Print summary
		fmt.Printf("Results for %s:\n", provider.Name)
		fmt.Printf("  Requests: %d\n", metrics.Requests)
		fmt.Printf("  Request Rate: %.2f/s\n", metrics.Rate)
		fmt.Printf("  Success Rate: %.2f%%\n", 100.0*metrics.Success)
		fmt.Printf("  Mean Latency: %s\n", metrics.Latencies.Mean)
		fmt.Printf("  P50 Latency: %s\n", metrics.Latencies.P50)
		fmt.Printf("  P99 Latency: %s\n", metrics.Latencies.P99)
		fmt.Printf("  Max Latency: %s\n", metrics.Latencies.Max)
		fmt.Printf("  Throughput: %.2f/s\n", metrics.Throughput)

		// Print server memory stats summary if available
		if len(serverMemStatsCopy) > 0 {
			var peakMem uint64
			for _, stat := range serverMemStatsCopy {
				if stat.RSS > peakMem {
					peakMem = stat.RSS
				}
			}
			fmt.Printf("  Server Peak Memory: %.2f MB\n\n", float64(peakMem)/(1024*1024))
		} else {
			fmt.Println("  No server memory statistics available")
		}

		// Apply cooldown period between tests (except after the last one)
		if i < len(providers)-1 && cooldown > 0 {
			fmt.Printf("Cooling down for %d seconds...\n", cooldown)
			time.Sleep(time.Duration(cooldown) * time.Second)
		}
	}

	return results
}

// getProcessByPort uses a more efficient approach to find a process by port
func getProcessByPort(port string) (*process.Process, error) {
	portNum, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid port number: %v", err)
	}

	// Get all processes and check their connections
	processes, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to get processes: %v", err)
	}

	for _, p := range processes {
		conns, err := p.Connections()
		if err != nil {
			continue
		}
		for _, conn := range conns {
			if conn.Laddr.Port == uint32(portNum) {
				fmt.Println("Found process for port:", port)
				return p, nil
			}
		}
	}

	return nil, fmt.Errorf("no process found listening on port %s", port)
}

// monitorServerMemory collects memory stats of the server process
func monitorServerMemory(p *process.Process, stop <-chan struct{}, stats *[]ServerMemStat, mutex *sync.Mutex) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			memInfo, err := p.MemoryInfo()
			if err != nil {
				continue
			}

			memPercent, err := p.MemoryPercent()
			if err != nil {
				memPercent = 0.0
			}

			memStat := ServerMemStat{
				Timestamp:  time.Now(),
				RSS:        memInfo.RSS, // Resident Set Size
				VMS:        memInfo.VMS, // Virtual Memory Size
				MemPercent: float64(memPercent),
			}

			mutex.Lock()
			*stats = append(*stats, memStat)
			mutex.Unlock()
		}
	}
}

func createTargeter(provider Provider) vegeta.Targeter {
	// Create a counter for round-robin message selection
	var requestCounter int64
	var counterMutex sync.Mutex

	return func(tgt *vegeta.Target) error {
		// Get next message index in round-robin fashion
		counterMutex.Lock()
		requestCounter++
		counterMutex.Unlock()

		// Create payload with the selected message
		var payload map[string]interface{}
		if err := json.Unmarshal(provider.Payload, &payload); err != nil {
			return err
		}

		text := payload["messages"].([]interface{})[0].(map[string]interface{})["content"].(string)

		// Replace placeholders with values
		updatedText := strings.ReplaceAll(text, "#{request_index}", fmt.Sprintf("%d", requestCounter))
		updatedText = strings.ReplaceAll(updatedText, "#{timestamp}", time.Now().Format(time.RFC3339))

		payload["messages"].([]interface{})[0].(map[string]interface{})["content"] = updatedText

		// Marshal the updated payload
		updatedPayload, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		tgt.Method = "POST"
		tgt.URL = provider.Endpoint
		tgt.Body = updatedPayload
		tgt.Header = http.Header{
			"Content-Type": []string{"application/json"},
		}

		if provider.Name == "Portkey" {
			openaiApiKey := os.Getenv("OPENAI_API_KEY")
			if openaiApiKey == "" {
				return fmt.Errorf("OPENAI_API_KEY is not set")
			}

			tgt.Header.Set("x-portkey-config", fmt.Sprintf(`{"provider":"openai","api_key":"%s"}`, openaiApiKey))
		}

		return nil
	}
}

func saveResults(results []BenchmarkResult, outputFile string) {
	// Convert results to a format suitable for JSON serialization
	// type SerializableMemStat struct {
	// 	Timestamp    string  `json:"timestamp"`
	// 	RSSBytes     uint64  `json:"rss_bytes"`
	// 	VMSBytes     uint64  `json:"vms_bytes"`
	// 	MemPercent   float64 `json:"mem_percent"`
	// 	RSSMegabytes float64 `json:"rss_mb"`
	// }

	type SerializableResult struct {
		Requests           uint64         `json:"requests"`
		Rate               float64        `json:"rate"` // Requests per second
		SuccessRate        float64        `json:"success_rate"`
		MeanLatencyMs      float64        `json:"mean_latency_ms"`
		P50LatencyMs       float64        `json:"p50_latency_ms"`
		P99LatencyMs       float64        `json:"p99_latency_ms"`
		MaxLatencyMs       float64        `json:"max_latency_ms"`
		ThroughputRPS      float64        `json:"throughput_rps"`
		Timestamp          string         `json:"timestamp"`
		ErrorRate          float64        `json:"error_rate"`
		StatusCodeCounts   map[string]int `json:"status_code_counts"`
		ServerPeakMemoryMB float64        `json:"server_peak_memory_mb"`
		ServerAvgMemoryMB  float64        `json:"server_avg_memory_mb"`
		// Include detailed memory stats
		// DetailedMemoryStats []SerializableMemStat `json:"detailed_memory_stats"`
	}

	// Create a map with provider names as keys
	resultsMap := make(map[string]SerializableResult)

	// Try to read existing results file
	if _, err := os.Stat(outputFile); err == nil {
		// File exists, read it
		fileData, err := os.ReadFile(outputFile)
		if err != nil {
			log.Printf("Warning: Could not read existing results file: %v", err)
		} else {
			// Parse existing results
			if err := json.Unmarshal(fileData, &resultsMap); err != nil {
				log.Printf("Warning: Could not parse existing results file: %v", err)
				resultsMap = make(map[string]SerializableResult)
			}
		}
	}

	// Update or add new results
	for _, res := range results {
		// Count status codes
		statusCodes := make(map[string]int)
		for code, count := range res.Metrics.StatusCodes {
			statusCodes[code] = int(count)
		}

		// Calculate peak and average server memory if available
		var peakMem uint64
		var totalMem uint64
		for _, stat := range res.ServerMemoryStats {
			if stat.RSS > peakMem {
				peakMem = stat.RSS
			}
			totalMem += stat.RSS
		}

		var avgMem float64
		if len(res.ServerMemoryStats) > 0 {
			avgMem = float64(totalMem) / float64(len(res.ServerMemoryStats)) / (1024 * 1024)
		}

		// Convert detailed memory stats
		// detailedStats := make([]SerializableMemStat, 0, len(res.ServerMemoryStats))
		// for _, stat := range res.ServerMemoryStats {
		// 	detailedStats = append(detailedStats, SerializableMemStat{
		// 		Timestamp:    stat.Timestamp.Format(time.RFC3339Nano),
		// 		RSSBytes:     stat.RSS,
		// 		VMSBytes:     stat.VMS,
		// 		MemPercent:   stat.MemPercent,
		// 		RSSMegabytes: float64(stat.RSS) / (1024 * 1024),
		// 	})
		// }

		resultsMap[strings.ToLower(res.ProviderName)] = SerializableResult{
			Requests:      res.Metrics.Requests,
			Rate:          res.Metrics.Rate,
			SuccessRate:   100.0 * res.Metrics.Success,
			MeanLatencyMs: float64(res.Metrics.Latencies.Mean) / float64(time.Millisecond),
			P50LatencyMs:  float64(res.Metrics.Latencies.P50) / float64(time.Millisecond),
			P99LatencyMs:  float64(res.Metrics.Latencies.P99) / float64(time.Millisecond),
			MaxLatencyMs:  float64(res.Metrics.Latencies.Max) / float64(time.Millisecond),
			ThroughputRPS: res.Metrics.Throughput,
			Timestamp:     time.Now().Format(time.RFC3339),
			ErrorRate: func() float64 {
				if res.Metrics.Requests > 0 {
					return 100.0 * float64(uint64(len(res.Metrics.StatusCodes)-1)) / float64(res.Metrics.Requests)
				}
				return 0.0
			}(),
			StatusCodeCounts:   statusCodes,
			ServerPeakMemoryMB: float64(peakMem) / (1024 * 1024),
			ServerAvgMemoryMB:  avgMem,
			// DetailedMemoryStats: detailedStats,
		}
	}

	// Serialize to JSON
	jsonData, err := json.MarshalIndent(resultsMap, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling results: %v", err)
	}

	// Write to file
	err = os.WriteFile(outputFile, jsonData, 0644)
	if err != nil {
		log.Fatalf("Error writing results to file: %v", err)
	}

	fmt.Printf("Results saved to %s\n", outputFile)
}
