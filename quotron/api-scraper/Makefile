.PHONY: build test clean

# Variables
MODULE = github.com/tiny-ria/quotron/api-scraper
BINNAME = api-scraper
MAINFILE = cmd/main/main.go

# Build the application
build:
	go build -o $(BINNAME) $(MAINFILE)

# Build and install in local bin directory
install:
	go install $(MAINFILE)

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f $(BINNAME)

# Run the application with Yahoo Finance (no API key required)
run-yahoo:
	go run $(MAINFILE) --yahoo

# Run the application with Alpha Vantage (requires API key)
run:
	go run $(MAINFILE)

# Display help
help:
	@echo "Quotron API Scraper"
	@echo ""
	@echo "make build      - Build the binary"
	@echo "make install    - Install binary to GOPATH/bin"
	@echo "make test       - Run tests"
	@echo "make clean      - Remove build artifacts"
	@echo "make run-yahoo  - Run with Yahoo Finance API"
	@echo "make run        - Run with Alpha Vantage API (requires API key)"