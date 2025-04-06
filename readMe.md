# Bifrost Benchmark Suite

This repository contains a set of APIs and benchmarking tools for comparing different LLM API gateways.

## Architecture

- **Bifrost API**: Go-based API running on a static port (3039), wrapped by a FastAPI server
- **OpenRouter API**: Python-based API for OpenRouter
- **LLMLite API**: Python-based API for LLMLite
- **Braintrust API**: Python-based API for Braintrust
- **Portkey API**: Python-based API for Portkey

## Setup

1. Install Go dependencies:
```
cd bifrost
go mod tidy
cd ..
```

2. Install Python dependencies:
```
pip install -r requirements.txt
```

3. Configure your `.env` file with your API keys and port settings:
```
OPENAI_API_KEY=your-openai-key
OPENROUTER_API_KEY=your-openrouter-key
BRAINTRUST_API_KEY=your-braintrust-key
BIFROST_PORT=3001
OPENROUTER_PORT=3002
LLMLITE_PORT=3003
BRAINTRUST_PORT=3004
PORTKEY_PORT=3005
```

## Running the APIs

You can run all APIs at once:
```
./run-servers.sh
```

Or run a specific API:
```
./run-servers.sh --server bifrost
./run-servers.sh --server openrouter
./run-servers.sh --server llmlite
./run-servers.sh --server braintrust
./run-servers.sh --server portkey
```

## Running the Benchmark

To benchmark all providers:
```
go run benchmark.go --rate 50 --duration 10
```

To benchmark a specific provider:
```
go run benchmark.go --rate 50 --duration 10 --provider bifrost
```

Results will be saved to `results.json` by default.

## Architecture Details

The Bifrost API is implemented as follows:
1. A FastAPI Python server listens on the configured port (e.g., 3001)
2. This server spawns a Go-based Bifrost server on the fixed port 3039
3. The FastAPI server forwards requests to the Go server and returns responses

This architecture allows the Bifrost API to have the same interface as other APIs while taking advantage of Go's performance benefits.

1. setting up the services
a. bifrost

b. portkey
    # Run the gateway locally (needs Node.js and npm)
    npx @portkey-ai/gateway

c. llmlite (runs locally on port 62312)
    
d. openrouter

e. 