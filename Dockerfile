# ─── Build Stage ───────────────────────────────────────────────────────────────
FROM golang:alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/lms-api ./cmd/api

# ─── Runtime Stage ─────────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/lms-api /usr/local/bin/lms-api

# Optimize memory usage for low-resource environments
ENV GOMEMLIMIT=250MiB
ENV GOGC=50

EXPOSE 8080

ENTRYPOINT ["lms-api"]
