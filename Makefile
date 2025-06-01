SHELL = sh.exe

# Имя бинарника
BINARY_NAME=registration-service

# --- Auth Proto ---
AUTH_PROTO_BASE_DIR=api/authproto
AUTH_PROTO_FILE=auth.proto
AUTH_PROTO_OUT_DIR=$(AUTH_PROTO_BASE_DIR)/proto-generate

# --- File Proto ---
FILE_PROTO_BASE_DIR=api/fileproto
FILE_PROTO_FILE=file.proto
FILE_PROTO_OUT_DIR=$(FILE_PROTO_BASE_DIR)/proto-generate

.PHONY: all build run clean test proto proto-auth proto-file fmt

all: build

build: proto
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) ./cmd/main

run: proto
	@echo "Running $(BINARY_NAME)..."
	go run ./cmd/main

# Общая цель для генерации всех proto
proto: proto-auth proto-file

proto-auth:
	@echo "Generating Go code for Auth proto..."
	@echo "Output directory for auth: $(AUTH_PROTO_OUT_DIR)"
	@mkdir -p $(AUTH_PROTO_OUT_DIR)
	@echo "Directory creation for auth attempted. Running protoc..."
	protoc \
		-I=$(AUTH_PROTO_BASE_DIR) \
		--go_out=$(AUTH_PROTO_OUT_DIR) \
		--go-grpc_out=$(AUTH_PROTO_OUT_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		$(AUTH_PROTO_FILE)

proto-file:
	@echo "Generating Go code for File proto..."
	@echo "Output directory for file: $(FILE_PROTO_OUT_DIR)"
	@mkdir -p $(FILE_PROTO_OUT_DIR)
	@echo "Directory creation for file attempted. Running protoc..."
	protoc \
		-I=$(FILE_PROTO_BASE_DIR) \
		--go_out=$(FILE_PROTO_OUT_DIR) \
		--go-grpc_out=$(FILE_PROTO_OUT_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		$(FILE_PROTO_FILE)

test:
	@echo "Running tests..."
	go test ./... -v -cover

fmt:
	@echo "Formatting Go files..."
	go fmt ./...

clean:
	@echo "Cleaning up..."
	@rm -f $(BINARY_NAME)
	@echo "Removing generated proto files..."
	@rm -rf $(AUTH_PROTO_OUT_DIR)
	@rm -rf $(FILE_PROTO_OUT_DIR)
	@echo "Clean finished."