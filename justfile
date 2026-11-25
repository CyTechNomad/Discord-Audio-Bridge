# Default recipe - list available commands
default:
    @just --list

# Format all code
format:
    gofmt -w *.go
    prettier --write static/*.html
    prettier --write .github/workflows/*.yml

# Build the application
build:
    go build -o discord-audio-bridge

# Run the application
run:
    go run main.go

# Update dependencies
deps:
    go get -u ./...
    go mod tidy
    go mod vendor

# Clean build artifacts
clean:
    rm -f discord-audio-bridge
    rm -rf dist/
