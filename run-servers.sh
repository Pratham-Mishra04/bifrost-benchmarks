#!/bin/bash

# Default values
SERVER="all"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --server)
            SERVER="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Load environment variables
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
else
    echo "Error: .env file not found"
    exit 1
fi

# Function to check if a port is in use
check_port() {
    if lsof -i :$1 > /dev/null 2>&1; then
        echo "Port $1 is already in use"
        return 1
    fi
    return 0
}

# Function to start a specific server
start_server() {
    local server_name=$1
    local port_var=$2
    local start_cmd=$3

    if [ "$SERVER" = "all" ] || [ "$SERVER" = "$server_name" ]; then
        echo "Starting $server_name API on port ${!port_var}..."
        eval "$start_cmd"
        return $!
    fi
    return 0
}

# Check required environment variables based on selected server
case $SERVER in
    "all")
        required_vars=("OPENAI_API_KEY" "OPENROUTER_API_KEY" "BRAINTRUST_API_KEY" "BIFROST_PORT" "OPENROUTER_PORT" "LLMLITE_PORT" "BRAINTRUST_PORT" "PORTKEY_PORT")
        ;;
    "bifrost")
        required_vars=("OPENAI_API_KEY" "BIFROST_PORT")
        ;;
    "openrouter")
        required_vars=("OPENROUTER_API_KEY" "OPENROUTER_PORT")
        ;;
    "llmlite")
        required_vars=("OPENAI_API_KEY" "LLMLITE_PORT")
        ;;
    "braintrust")
        required_vars=("BRAINTRUST_API_KEY" "BRAINTRUST_PORT")
        ;;
    "portkey")
        required_vars=("OPENAI_API_KEY" "PORTKEY_PORT")
        ;;
    *)
        echo "Invalid server name. Valid options are: all, bifrost, openrouter, llmlite, braintrust, portkey"
        exit 1
        ;;
esac

# Check required environment variables
for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo "Error: $var is not set in .env file"
        exit 1
    fi
done

# Check ports based on selected server
case $SERVER in
    "all")
        check_port $BIFROST_PORT || exit 1
        check_port $OPENROUTER_PORT || exit 1
        check_port $LLMLITE_PORT || exit 1
        check_port $BRAINTRUST_PORT || exit 1
        check_port $PORTKEY_PORT || exit 1
        ;;
    "bifrost")
        check_port $BIFROST_PORT || exit 1
        ;;
    "openrouter")
        check_port $OPENROUTER_PORT || exit 1
        ;;
    "llmlite")
        check_port $LLMLITE_PORT || exit 1
        ;;
    "braintrust")
        check_port $BRAINTRUST_PORT || exit 1
        ;;
    "portkey")
        check_port $PORTKEY_PORT || exit 1
        ;;
esac

# Start servers based on selection
if [ "$SERVER" = "all" ] || [ "$SERVER" = "bifrost" ]; then
    cd bifrost
    echo "Starting Bifrost API Wrapper on port $BIFROST_PORT..."
    python api.py --openai-key $OPENAI_API_KEY --port $BIFROST_PORT &
    BIFROST_PID=$!
fi

if [ "$SERVER" = "all" ] || [ "$SERVER" = "openrouter" ]; then
    cd openrouter
    python api.py --openrouter-key $OPENROUTER_API_KEY --port $OPENROUTER_PORT &
    OPENROUTER_PID=$!
    cd ..
fi

if [ "$SERVER" = "all" ] || [ "$SERVER" = "llmlite" ]; then
    cd llmlite
    python api.py --openai-key $OPENAI_API_KEY --port $LLMLITE_PORT &
    LLMLITE_PID=$!
    cd ..
fi

if [ "$SERVER" = "all" ] || [ "$SERVER" = "braintrust" ]; then
    cd braintrust
    python api.py --braintrust-key $BRAINTRUST_API_KEY --port $BRAINTRUST_PORT &
    BRAINTRUST_PID=$!
    cd ..
fi

if [ "$SERVER" = "all" ] || [ "$SERVER" = "portkey" ]; then
    echo "Starting Portkey Gateway on port 8787..."
    npx @portkey-ai/gateway &
    PORTKEY_GATEWAY_PID=$!

    cd portkey
    python api.py --openai-key $OPENAI_API_KEY --port $PORTKEY_PORT &
    PORTKEY_PID=$!
    cd ..
fi

# Function to handle cleanup
cleanup() {
    echo "Shutting down servers..."
    if [ "$SERVER" = "all" ] || [ "$SERVER" = "bifrost" ]; then
        kill $BIFROST_PID 2>/dev/null
    fi
    if [ "$SERVER" = "all" ] || [ "$SERVER" = "openrouter" ]; then
        kill $OPENROUTER_PID 2>/dev/null
    fi
    if [ "$SERVER" = "all" ] || [ "$SERVER" = "llmlite" ]; then
        kill $LLMLITE_PID 2>/dev/null
    fi
    if [ "$SERVER" = "all" ] || [ "$SERVER" = "braintrust" ]; then
        kill $BRAINTRUST_PID 2>/dev/null
    fi
    if [ "$SERVER" = "all" ] || [ "$SERVER" = "portkey" ]; then
        kill $PORTKEY_PID $PORTKEY_GATEWAY_PID 2>/dev/null
    fi
    exit 0
}

# Set up trap for cleanup
trap cleanup SIGINT SIGTERM

# Wait for all processes
wait
