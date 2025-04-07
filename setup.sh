#!/bin/bash

echo "Setting up Bifrost Benchmarks environment..."

# Function to detect if Go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        echo "Error: Go is not installed. Please install Go before continuing."
        exit 1
    fi
}

# Function to detect if Python is installed
check_python() {
    if ! command -v python3 &> /dev/null; then
        echo "Error: Python 3 is not installed. Please install Python 3 before continuing."
        exit 1
    fi
}

# Function to detect if Node.js is installed
check_node() {
    if ! command -v node &> /dev/null; then
        echo "Error: Node.js is not installed. Please install Node.js before continuing."
        exit 1
    fi
    
    if ! command -v npx &> /dev/null; then
        echo "Error: npx is not available. Please ensure you have a recent version of Node.js."
        exit 1
    fi
}

# Function to install Go dependencies
install_go_deps() {
    local dir=$1
    echo "Installing Go dependencies in $dir..."
    
    if [ -f "$dir/go.mod" ]; then
        cd "$dir"
        go mod download
        go mod tidy
        cd - > /dev/null
    fi
}

# Function to install Python dependencies
install_python_deps() {
    local dir=$1
    echo "Installing Python dependencies in $dir..."
    
    if [ -f "$dir/requirements.txt" ]; then
        cd "$dir"
        python3 -m pip install -r requirements.txt
        cd - > /dev/null
    fi
}

# Check required tools
check_go
check_python
check_node

# Install Go dependencies in root and bifrost directories
echo "Installing Go dependencies..."
install_go_deps "."
install_go_deps "bifrost"

# Python directories
directories=("bifrost" "openrouter" "llmlite" "braintrust" "portkey")

# Install Python dependencies
for dir in "${directories[@]}"; do
    echo "Setting up Python dependencies in $dir..."
    install_python_deps "$dir"
done

echo "Setup completed successfully!"
echo "You can now run the servers using ./run-servers.sh" 