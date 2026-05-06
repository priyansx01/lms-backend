# ─── Build Stage ───────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/lms-api ./cmd/api

# ─── Runtime Stage ─────────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/lms-api /usr/local/bin/lms-api

EXPOSE 8080

ENTRYPOINT ["lms-api"]
