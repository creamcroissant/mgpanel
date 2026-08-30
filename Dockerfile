# syntax=docker/dockerfile:1.7-labs

# =============================================================================
# Stage 1: Frontend Build (Unified User + Admin)
# =============================================================================
FROM node:20-alpine AS frontend

WORKDIR /frontend

# User Frontend (includes Admin interface)
COPY web/user-vite/package*.json ./user/
RUN --mount=type=cache,target=/root/.npm \
    cd user && npm ci --silent

COPY web/user-vite ./user/
RUN cd user && npm run build

# =============================================================================
# Stage 2: Backend Build
# =============================================================================
FROM golang:1.25-alpine AS backend

ARG TARGETOS=linux
ARG TARGETARCH

RUN apk add --no-cache gcc musl-dev git

WORKDIR /src

# Download dependencies first (for better caching)
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Copy built frontend assets (unified user-vite)
COPY --from=frontend /frontend/user/dist ./web/user-vite/dist

# Build with version info
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o /out/xboard ./cmd/xboard

# =============================================================================
# Stage 3: Final Image
# =============================================================================
FROM alpine:3.20

ARG TARGETOS=linux
ARG TARGETARCH

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=backend /out/xboard /usr/local/bin/xboard

# Default environment variables
ENV XBOARD_DB_PATH=/data/xboard.db
ENV XBOARD_HTTP_ADDR=:8080

# Data volume for SQLite database
VOLUME ["/data"]

# Config volume (optional)
VOLUME ["/etc/xboard"]

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/xboard"]
CMD ["serve", "--config", "/etc/xboard/config.yml"]
