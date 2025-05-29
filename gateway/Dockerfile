FROM golang:latest

WORKDIR /app

# Copy the main module files
COPY go.mod go.sum ./

# Copy gateway module files
COPY gateway/go.mod gateway/go.sum gateway/

# Copy source code
COPY api/ api/
COPY gateway/ gateway/

# Set working directory to gateway
WORKDIR /app/gateway

# Download all dependencies
RUN go mod download

# Build the application
RUN go build -o main .

# Expose port
EXPOSE 8080

# Command to run the executable
CMD ["./main"] 