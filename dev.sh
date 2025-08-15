#!/bin/bash

# Development helper functions for resim CLI

# Build the resim binary
build() {
    echo "Building resim binary..."
    go build -o resim ./cmd/resim
    if [ $? -eq 0 ]; then
        echo "Build successful! Binary created as 'resim'"
    else
        echo "Build failed!"
        return 1
    fi
}

# Run the test command script
runCommand() {
    if [ ! -f "./runCommand.sh" ]; then
        echo "Error: runCommand.sh not found!"
        echo "Create runCommand.sh with your test commands"
        return 1
    fi
    
    echo "Running test command..."
    ./runCommand.sh
}

# Show available commands
help() {
    echo "Available commands:"
    echo "  build      - Build the resim binary (go build -o resim ./cmd/resim)"
    echo "  runCommand - Run ./runCommand.sh"
    echo "  help       - Show this help message"
}

# Export functions so they're available when sourced
export -f build
export -f runCommand
export -f help
