# ───── Stage 1: Build ─────
FROM golang:1.24 as builder

WORKDIR /app

# Copy go source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the Go binary
RUN go build -o webhook-tester ./cmd/server/main.go

# ───── Stage 2: Final ─────
FROM golang:1.24

WORKDIR /app

# Copy the built binary
COPY --from=builder /app/webhook-tester .

# Copy static assets, migrations
COPY static/ static/
COPY db/migrations/ db/migrations/

# If using a default .env file
# COPY .env .

# Expose port (adjust if needed)
EXPOSE 3000

# Command to run
CMD ["./webhook-tester"]