# Multi-stage build for Quotron services

# Golang API Scraper build
FROM golang:1.21-alpine AS go-builder
WORKDIR /app
COPY api-scraper/go.mod api-scraper/go.sum* ./
RUN go mod download
COPY api-scraper/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /quotron-api-scraper ./cmd/main

# Golang Scheduler build
FROM golang:1.21-alpine AS scheduler-builder
WORKDIR /app
COPY scheduler/go.mod scheduler/go.sum* ./
RUN go mod download
COPY scheduler/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /quotron-scheduler ./cmd/scheduler

# Python base
FROM python:3.11-slim AS py-base
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app

# Final images
# API Scraper
FROM alpine:3.18 AS api-scraper
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=go-builder /quotron-api-scraper .
ENTRYPOINT ["./quotron-api-scraper"]

# Scheduler
FROM alpine:3.18 AS scheduler
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=scheduler-builder /quotron-scheduler .
ENTRYPOINT ["./quotron-scheduler"]

# Use specific service by specifying target during build
# e.g., docker build --target api-scraper -t quotron-api-scraper .