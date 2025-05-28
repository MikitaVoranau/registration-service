FROM golang:latest

WORKDIR /app

# Copy go mod and sum files
COPY gateway/go.mod gateway/go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY gateway/ ./

# Build the application
RUN go build -o main .

# Expose port
EXPOSE 8080

# Command to run the executable
CMD ["./main"] 