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
	"github.com/shirou/gopsutil/net"
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
	ServerMemoryStats []ServerMemStat
	DropReasons       map[string]int // Track reasons for dropped requests
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
	duration := flag.Int("duration", 10, "Duration of test in seconds")
	outputFile := flag.String("output", "results.json", "Output file for results")
	cooldown := flag.Int("cooldown", 60, "Cooldown period between tests in seconds")
	provider := flag.String("provider", "", "Specific provider to benchmark (bifrost, portkey, braintrust, llmlite, openrouter)")
	bigPayload := flag.Bool("big-payload", false, "Use a bigger payload")
	model := flag.String("model", "gpt-4o-mini", "Model to use")
	suffix := flag.String("suffix", "v1", "Suffix to add to the url route")

	flag.Parse()

	// Initialize providers
	providers := initializeProviders(*bigPayload, *model, *suffix)

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

func initializeProviders(bigPayload bool, model string, suffix string) []Provider {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	var payload []byte

	if bigPayload {
		// Create payload template with dynamic content placeholders
		payload, _ = json.Marshal(map[string]interface{}{
			"messages": []map[string]string{
				{
					"role": "user",
					"content": "This is a benchmark request #{request_index} at #{timestamp}. " +
						"Please provide a comprehensive analysis of the following topics: " +
						"1. Explain the concept of Proxy Gateway in the context of AI, including its architecture, benefits, and use cases. " +
						"2. Discuss the role of load balancing and request routing in AI proxy gateways. " +
						"3. Analyze the impact of caching and rate limiting on AI service performance. " +
						"4. Describe common challenges in implementing AI proxy gateways and potential solutions. " +
						"5. Compare different AI proxy gateway implementations and their trade-offs. " +
						"6. What is the difference between a proxy gateway and a reverse proxy? " +
						"7. What is the difference between a proxy gateway and a load balancer? " +
						"8. What is the difference between a proxy gateway and a web server? " +
						"9. What is the difference between a proxy gateway and a CDN? " +
						"10. What is the difference between a proxy gateway and a firewall? " +
						"11. What is the difference between a proxy gateway and a VPN? " +
						"12. What is the difference between a proxy gateway and a WAF? " +
						"13. What is the difference between a proxy gateway and a DDoS protection service? " +
						"14. What is the difference between a proxy gateway and a DNS server? " +
						"15. What is the difference between a proxy gateway and a web application firewall? " +
						"16. What is the difference between a proxy gateway and a load balancer? " +
						"17. What is the difference between a proxy gateway and a web server? " +
						"18. What is the difference between a proxy gateway and a CDN? " +
						"19. What is the difference between a proxy gateway and a firewall? " +
						"20. What is the difference between a proxy gateway and a VPN? " +
						"Please provide detailed explanations with examples and technical details for each point. ",
				},
			},
			"provider": "openai",
			"model":    model,
		})
	} else {
		payload, _ = json.Marshal(map[string]interface{}{
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "This is a benchmark request #{request_index} at #{timestamp}. How are you?",
				},
			},
			"provider": "openai",
			"model":    model,
		})
	}

	baseUrl := "http://localhost:%s/%s/chat/completions"

	// Create providers with ports from .env
	providers := []Provider{
		{
			Name:     "Bifrost",
			Endpoint: fmt.Sprintf(baseUrl, os.Getenv("BIFROST_PORT"), suffix),
			Port:     os.Getenv("BIFROST_PORT"),
			Payload:  payload,
		},
		{
			Name:     "Litellm",
			Endpoint: fmt.Sprintf(baseUrl, os.Getenv("LITELLM_PORT"), suffix),
			Port:     os.Getenv("LITELLM_PORT"),
			Payload:  payload,
		},
		// {
		// 	Name:     "Portkey",
		// 	Endpoint: fmt.Sprintf(baseUrl, os.Getenv("PORTKEY_PORT")),
		// 	Port:     os.Getenv("PORTKEY_PORT"),
		// 	Payload:  payload,
		// },
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
		{
			Name:     "Helicone",
			Endpoint: fmt.Sprintf(baseUrl, os.Getenv("HELICONE_PORT"), suffix),
			Port:     os.Getenv("HELICONE_PORT"),
			Payload:  payload,
		},
	}

	return providers
}

func runBenchmarks(providers []Provider, rate int, duration int, cooldown int) []BenchmarkResult {
	results := make([]BenchmarkResult, 0, len(providers))

	for i, provider := range providers {
		fmt.Printf("Benchmarking %s...\n", provider.Name)

		httpTransport := &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConnsPerHost: 100000,
			MaxConnsPerHost:     0,
			IdleConnTimeout:     10 * time.Second,
			// Optionally tune TLS and other settings if needed
		}

		httpClient := &http.Client{
			Transport: httpTransport,
			Timeout:   240 * time.Second, // adjust as necessary
		}

		// Define the attack
		targeter := createTargeter(provider)
		attacker := vegeta.NewAttacker(vegeta.Client(httpClient))

		// Setup memory monitoring for the server
		var serverMemStats []ServerMemStat
		var memMutex sync.Mutex
		stopMonitoring := make(chan struct{})
		var wg sync.WaitGroup

		// Initialize drop reasons tracking
		dropReasons := make(map[string]int)

		// Start server memory monitoring
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := getProcessByPort(provider.Port)
			if err != nil {
				log.Printf("Warning: Could not find process on port %s: %v", provider.Port, err)
				return
			}

			monitorServerMemory(p, stopMonitoring, &serverMemStats, &memMutex)
		}()

		// Create context with timeout for the attack
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(240)*time.Second) // Changed to 240s
		defer cancel()

		// Run the benchmark
		var metrics vegeta.Metrics
		attackRate := vegeta.Rate{Freq: rate, Per: time.Second}
		for res := range attacker.Attack(targeter, attackRate, time.Duration(duration)*time.Second, provider.Name) {
			metrics.Add(res)

			// Track drop reasons
			if res.Error != "" {
				dropReasons[res.Error]++
			} else if res.Code != 200 {
				dropReasons[fmt.Sprintf("HTTP %d", res.Code)]++
			}

			// Check if context is done
			select {
			case <-ctx.Done():
				log.Printf("Attack for %s timed out", provider.Name)
				dropReasons["context_timeout"]++
				goto EndAttack
			default:
				// Continue with the attack
			}
		}

	EndAttack:
		metrics.Close()

		// Stop memory monitoring
		close(stopMonitoring)
		wg.Wait()

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
			DropReasons:       dropReasons,
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

	conns, err := net.Connections("tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to get connections: %v", err)
	}

	for _, conn := range conns {
		if conn.Laddr.Port == uint32(portNum) && conn.Status == "LISTEN" {
			p, err := process.NewProcess(conn.Pid)
			if err != nil {
				continue
			}
			cmdline, _ := p.Cmdline()
			fmt.Printf("Found process on port %s: PID=%d, Cmdline=%s\n", port, conn.Pid, cmdline)
			return p, nil
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
	type SerializableResult struct {
		Requests           uint64         `json:"requests"`
		Rate               float64        `json:"rate"`
		SuccessRate        float64        `json:"success_rate"`
		MeanLatencyMs      float64        `json:"mean_latency_ms"`
		P50LatencyMs       float64        `json:"p50_latency_ms"`
		P99LatencyMs       float64        `json:"p99_latency_ms"`
		MaxLatencyMs       float64        `json:"max_latency_ms"`
		ThroughputRPS      float64        `json:"throughput_rps"`
		Timestamp          string         `json:"timestamp"`
		StatusCodeCounts   map[string]int `json:"status_code_counts"`
		ServerPeakMemoryMB float64        `json:"server_peak_memory_mb"`
		ServerAvgMemoryMB  float64        `json:"server_avg_memory_mb"`
		DropReasons        map[string]int `json:"drop_reasons"` // Add drop reasons to serialized output
	}

	// Create a map with provider names as keys
	resultsMap := make(map[string]SerializableResult)

	// Try to read existing results file
	if _, err := os.Stat(outputFile); err == nil {
		fileData, err := os.ReadFile(outputFile)
		if err != nil {
			log.Printf("Warning: Could not read existing results file: %v", err)
		} else {
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

		resultsMap[strings.ToLower(res.ProviderName)] = SerializableResult{
			Requests:           res.Metrics.Requests,
			Rate:               res.Metrics.Rate,
			SuccessRate:        100.0 * res.Metrics.Success,
			MeanLatencyMs:      float64(res.Metrics.Latencies.Mean) / float64(time.Millisecond),
			P50LatencyMs:       float64(res.Metrics.Latencies.P50) / float64(time.Millisecond),
			P99LatencyMs:       float64(res.Metrics.Latencies.P99) / float64(time.Millisecond),
			MaxLatencyMs:       float64(res.Metrics.Latencies.Max) / float64(time.Millisecond),
			ThroughputRPS:      res.Metrics.Throughput,
			Timestamp:          time.Now().Format(time.RFC3339),
			StatusCodeCounts:   statusCodes,
			ServerPeakMemoryMB: float64(peakMem) / (1024 * 1024),
			ServerAvgMemoryMB:  avgMem,
			// DropReasons:        res.DropReasons, // Include drop reasons in output
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
