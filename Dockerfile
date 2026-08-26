# ───── Stage 1: Build CSS ─────
FROM alpine:3.20 AS css-builder

WORKDIR /app

RUN apk add --no-cache curl && \
    curl -sLo /usr/local/bin/tailwindcss https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.17/tailwindcss-linux-x64 && \
    chmod +x /usr/local/bin/tailwindcss

COPY tailwind.config.js ./
COPY static/css/input.css static/css/input.css
COPY internal/web/templates/ internal/web/templates/

RUN tailwindcss -i static/css/input.css -o static/css/tailwind.css --minify

# ───── Stage 2: Build Go binary ─────
FROM golang:1.27-alpine AS base

WORKDIR /app

COPY . .

RUN go mod download

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o webhook-tester ./cmd/main.go

# ───── Stage 3: Final ─────
FROM scratch

WORKDIR /app

# Copy the built binary
COPY --from=base /app/webhook-tester .

# Copy static assets, migrations and docs
COPY static/ static/
COPY --from=css-builder /app/static/css/tailwind.css static/css/tailwind.css
COPY db/migrations/ db/migrations/
COPY docs/ docs/

EXPOSE 3000

# Command to run
CMD ["./webhook-tester"]