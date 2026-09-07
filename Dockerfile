FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev upx

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /tmp/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o /tmp/client.exe ./cmd/client && \
    upx --best /tmp/server /tmp/client.exe 2>/dev/null || true && \
    cp config.txt /tmp/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates upx

WORKDIR /app

COPY --from=builder /tmp/server ./server
COPY --from=builder /tmp/client.exe ./client.exe
COPY --from=builder /tmp/config.txt ./config.txt

EXPOSE 8182

ENV PORT=8182

ENTRYPOINT ["./server"]