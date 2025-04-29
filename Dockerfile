# ───── Stage 1: Build ─────
FROM golang:1.24-alpine AS base

WORKDIR /app

COPY . .

RUN go mod download

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o webhook-tester ./cmd/main.go

# ───── Stage 2: Final ─────
FROM scratch

WORKDIR /app

# Copy the built binary
COPY --from=base /app/webhook-tester .

# Copy static assets, migrations
COPY static/ static/
COPY db/migrations/ db/migrations/

EXPOSE 3000

# Command to run
CMD ["./webhook-tester"]