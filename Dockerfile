FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build server binary
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/server ./cmd/server

# Build Windows client binary
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o bin/client.exe ./cmd/client

# Copy config
RUN cp config.txt bin/

# Final stage - minimal runtime
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy built binaries and config
COPY --from=builder /app/bin/server ./server
COPY --from=builder /app/bin/client.exe ./client.exe
COPY --from=builder /app/bin/config.txt ./config.txt

EXPOSE 8182

ENTRYPOINT ["./server"]