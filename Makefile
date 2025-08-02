.PHONY: all build test clean cert run

# Default target
all: clean cert build test

# Build the project
build:
	@echo "Building Punto accesso server..."
	@cd pec-server/punto-accesso && go build -v -o pec-punto-accesso .

	@echo "Building Punto ricezione server..."
	@cd pec-server/punto-ricezione && go build -v -o pec-punto-ricezione .

	@echo "Building Punto consegna server..."
	@cd pec-server/punto-consegna && go build -v -o pec-punto-consegna .

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f pec-server/punto-accesso/pec-punto-accesso
	@rm -f pec-server/punto-accesso/pec.log
	@rm -f pec-server/punto-ricezione/pec-punto-ricezione
	@rm -f pec-server/punto-ricezione/pec.log
	@rm -f pec-server/punto-consegna/pec-punto-consegna
	@rm -f pec-server/punto-consegna/pec.log

# Generate certificates
cert:
	@echo "Generating certificates..."
	@cd pec-server && openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem \
		-days 365 -nodes -subj "/C=IT/O=PEC Test/CN=posta-certificata.local" \
		-addext "extendedKeyUsage=emailProtection" \
		-addext "subjectAltName=email:posta-certificata@localhost"

# Run the server
run: build
	@echo "Starting Punto consegna server..."
	@cd pec-server/punto-consegna && ./pec-punto-consegna

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download

# Migrations
migrate-up:
	migrate -path pec-server/internal/storage/migrations -database "postgres://$$PGUSER:$$PGPASSWORD@$$PGHOST:$$PGPORT/$$PGDATABASE?sslmode=require" up

migrate-down:
	migrate -path pec-server/internal/storage/migrations -database "postgres://$$PGUSER:$$PGPASSWORD@$$PGHOST:$$PGPORT/$$PGDATABASE?sslmode=require" down

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