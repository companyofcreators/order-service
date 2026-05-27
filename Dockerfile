FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/server ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /build/server .
COPY --from=builder /build/migrations ./migrations

EXPOSE 8082

USER appuser

HEALTHCHECK --interval=15s --timeout=5s --retries=3 \
    CMD curl -f http://localhost:8082/internal/health || exit 1

ENTRYPOINT ["./server"]
