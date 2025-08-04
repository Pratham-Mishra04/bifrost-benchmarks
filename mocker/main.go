package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

type OpenAIResponse struct {
	ID                string                          `json:"id"`                 // Unique identifier for the completion
	Object            string                          `json:"object"`             // Type of completion (text.completion or chat.completion)
	Choices           []schemas.BifrostResponseChoice `json:"choices"`            // Array of completion choices
	Model             string                          `json:"model"`              // Model used for the completion
	Created           int                             `json:"created"`            // Unix timestamp of completion creation
	ServiceTier       *string                         `json:"service_tier"`       // Service tier used for the request
	SystemFingerprint *string                         `json:"system_fingerprint"` // System fingerprint for the request
	Usage             schemas.LLMUsage                `json:"usage"`              // Token usage statistics
}

// OpenAIError represents the error response structure from the OpenAI API.
// It includes detailed error information and event tracking.
type OpenAIError struct {
	EventID string `json:"event_id"` // Unique identifier for the error event
	Type    string `json:"type"`     // Type of error
	Error   struct {
		Type    string      `json:"type"`     // Error type
		Code    string      `json:"code"`     // Error code
		Message string      `json:"message"`  // Error message
		Param   interface{} `json:"param"`    // Parameter that caused the error
		EventID string      `json:"event_id"` // Event ID for tracking
	} `json:"error"`
}

var (
	port       int
	latency    int
	bigPayload bool
)

func init() {
	flag.IntVar(&port, "port", 8000, "Port for the mock server to listen on")
	flag.IntVar(&latency, "latency", 0, "Latency in milliseconds to simulate")
	flag.BoolVar(&bigPayload, "big-payload", false, "Use big payload")
}

// StrPtr creates a pointer to a string value.
func StrPtr(s string) *string {
	return &s
}

func mockOpenAIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simulate latency
	if latency > 0 {
		time.Sleep(time.Duration(latency) * time.Millisecond)
	}

	mockContent := "This is a mocked response from the OpenAI mocker server."
	if bigPayload {
		// Repeat content to generate approximately 10KB response
		// Each repetition is ~55 chars, so ~182 repetitions ≈ 10KB
		mockContent = strings.Repeat(mockContent, 182)
	}

	// Create a mock response
	mockChoiceMessage := schemas.BifrostResponseChoiceMessage{
		Role:    schemas.ModelChatMessageRole("assistant"),
		Content: StrPtr(mockContent),
	}
	mockChoice := schemas.BifrostResponseChoice{
		Index:        0,
		Message:      mockChoiceMessage,
		FinishReason: StrPtr("stop"),
	}

	randomInputTokens := rand.Intn(1000)
	randomOutputTokens := rand.Intn(1000)

	mockResp := OpenAIResponse{
		ID:      "cmpl-mock12345",
		Object:  "chat.completion",
		Created: int(time.Now().Unix()),
		Model:   "gpt-3.5-turbo-mock",
		Choices: []schemas.BifrostResponseChoice{mockChoice},
		Usage: schemas.LLMUsage{
			PromptTokens:     randomInputTokens,
			CompletionTokens: randomOutputTokens,
			TotalTokens:      randomInputTokens + randomOutputTokens,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(mockResp); err != nil {
		log.Printf("Error encoding mock response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func main() {
	flag.Parse()

	http.HandleFunc("/v1/chat/completions", mockOpenAIHandler)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Mock OpenAI server starting on port %d with latency %dms...\n", port, latency)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
