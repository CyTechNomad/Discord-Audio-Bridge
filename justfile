# Default recipe - list available commands
default:
    @just --list

# Format all code
format:
    gofmt -w *.go
    gofmt -w internal/
    prettier --write static/*.html
    prettier --write .github/workflows/*.yml

# Build the application
build:
    go build -o discord-audio-bridge

# Run the application
run:
    go run main.go

# Run tests
test:
    go test ./... -v

# Run tests with coverage
test-coverage:
    go test ./... -coverprofile=coverage.out
    go tool cover -html=coverage.out -o coverage.html

# Update dependencies
deps:
    go get -u ./...
    go mod tidy
    go mod vendor

# Clean build artifacts
clean:
    rm -f discord-audio-bridge
    rm -rf dist/
    rm -f coverage.out coverage.html
