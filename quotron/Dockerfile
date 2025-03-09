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

# Browser Scraper build
FROM py-base AS browser-scraper
COPY browser-scraper/playwright/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY browser-scraper/playwright/ .
# Install browser dependencies
RUN python -m playwright install --with-deps chromium
ENTRYPOINT ["python", "src/scraper.py"]

# Auth Engine build
FROM py-base AS auth-engine
COPY auth-engine/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY auth-engine/ .
ENTRYPOINT ["python", "service/auth_service.py"]

# Ingest Pipeline build
FROM py-base AS ingest-pipeline
COPY ingest-pipeline/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY ingest-pipeline/ .

# Events System build
FROM py-base AS events
COPY events/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY events/ .

# Dashboard build
FROM py-base AS dashboard-builder
COPY dashboard/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY dashboard/ .

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

# Dashboard
FROM py-base AS dashboard
COPY --from=dashboard-builder /app /app/dashboard
# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    git \
    && rm -rf /var/lib/apt/lists/*
# Install Go for scheduler control
RUN curl -OL https://golang.org/dl/go1.21.5.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz \
    && rm go1.21.5.linux-amd64.tar.gz
ENV PATH=$PATH:/usr/local/go/bin
WORKDIR /app/dashboard
ENTRYPOINT ["streamlit", "run", "dashboard.py", "--server.port=8501", "--server.address=0.0.0.0"]

# Use specific service by specifying target during build
# e.g., docker build --target api-scraper -t quotron-api-scraper .