# Имя бинарника
BINARY_NAME=registration-service

# Директория, где находится proto-файл
PROTO_DIR=api
PROTO_FILE=$(PROTO_DIR)/auth.proto

# Куда генерировать .pb.go файлы
PROTO_OUT_DIR=api/proto-generate

.PHONY: all build run clean test proto fmt

build:
	go build -o $(BINARY_NAME) ./cmd/main

run:
	go run ./cmd/main

proto:
	protoc \
		-I=$(PROTO_DIR) \
		--go_out=$(PROTO_OUT_DIR) \
		--go-grpc_out=$(PROTO_OUT_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)

test:
	go test ./... -v -cover

fmt:
	go fmt ./...

clean:
	rm -f $(BINARY_NAME)
