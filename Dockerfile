# Gopedia Fuego API Server - Multi-stage build
# Stage 1: Build the Go API server
FROM golang:1.24-alpine AS builder
WORKDIR /build

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Build the api binary
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api

# Stage 2: Python environment with API binary
FROM python:3.12-slim
WORKDIR /app

# Install python dependencies for Xylem / Phloem ingest
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
RUN pip install --no-cache-dir --upgrade 'protobuf>=4.25'

# Copy the built Go binary
COPY --from=builder /api /usr/local/bin/api

# The application code should be mounted to /app at runtime,
# or we can copy it here for a self-contained image.
# We'll copy it so the image is fully usable even without mounts,
# but docker-compose mounts . to /app during development.
COPY . .

EXPOSE 18787
ENTRYPOINT ["/usr/local/bin/api"]
