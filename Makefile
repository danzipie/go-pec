.PHONY: all build test clean cert run

# Default target
all: clean cert build test

# Build the project
build:
	@echo "Building PEC server..."
	@cd pec-server && go build -v -o pec-server .

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f pec-server/pec-server
	@rm -f pec-server/pec.log

# Generate certificates
cert:
	@echo "Generating certificates..."
	@cd pec-server && openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem \
		-days 365 -nodes -subj "/C=IT/O=PEC Test/CN=posta-certificata.local" \
		-addext "extendedKeyUsage=emailProtection" \
		-addext "subjectAltName=email:posta-certificata@localhost"

# Run the server
run: build
	@echo "Starting PEC server..."
	@cd pec-server && ./pec-server

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download

# Help target
help:
	@echo "Available targets:"
	@echo "  make          : Build everything (clean, generate certs, build, test)"
	@echo "  make build    : Build the PEC server"
	@echo "  make test     : Run tests"
	@echo "  make clean    : Remove build artifacts"
	@echo "  make cert     : Generate SSL certificates"
	@echo "  make run      : Build and run the PEC server"
	@echo "  make deps     : Install dependencies"
	@echo "  make help     : Show this help message" 