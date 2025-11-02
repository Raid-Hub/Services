# Build stage: Build all cron tool binaries and cron manager
FROM golang:1.25-alpine AS build

# Install sqlite3 development libraries and build tools for manifest-downloader (requires CGO)
RUN apk add --no-cache \
    sqlite-dev \
    gcc \
    musl-dev

WORKDIR /src

# Pre-cache modules separately for faster rebuilds
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy all tools, lib, and cron manager
COPY tools/ ./tools/
COPY lib/ ./lib/
COPY infrastructure/cron/manager/ ./infrastructure/cron/manager/

# Build all tools and cron manager
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /RaidHub/bin && \
    CGO_ENABLED=1 GOOS=linux go build -o /out/ ./tools/... && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/cron-manager ./infrastructure/cron/manager/

# Final stage: Cron daemon + Manager
FROM alpine:latest

# Install cron, shell, and required packages
RUN apk add --no-cache dcron tzdata bash supervisor

# Copy binaries from build stage
COPY --from=build /out/ /RaidHub/bin/

# Copy cron manager binary separately
COPY --from=build /out/cron-manager /usr/local/bin/cron-manager

# Copy HTML file and static assets for cron manager
RUN mkdir -p /usr/local/share/cron-manager
COPY infrastructure/cron/manager/index.html /usr/local/share/cron-manager/index.html
COPY infrastructure/cron/manager/styles.css /usr/local/share/cron-manager/styles.css
COPY infrastructure/cron/manager/app.js /usr/local/share/cron-manager/app.js

# Create log directories
RUN mkdir -p /var/logs /var/log/cron

# Create supervisor config for running both cron and cron-manager
RUN echo "[supervisord]" > /etc/supervisord.conf && \
    echo "nodaemon=true" >> /etc/supervisord.conf && \
    echo "" >> /etc/supervisord.conf && \
    echo "[program:cron]" >> /etc/supervisord.conf && \
    echo "command=crond -f -d 8" >> /etc/supervisord.conf && \
    echo "autostart=true" >> /etc/supervisord.conf && \
    echo "autorestart=true" >> /etc/supervisord.conf && \
    echo "stdout_logfile=/dev/stdout" >> /etc/supervisord.conf && \
    echo "stdout_logfile_maxbytes=0" >> /etc/supervisord.conf && \
    echo "stderr_logfile=/dev/stderr" >> /etc/supervisord.conf && \
    echo "stderr_logfile_maxbytes=0" >> /etc/supervisord.conf && \
    echo "" >> /etc/supervisord.conf && \
    echo "[program:cron-manager]" >> /etc/supervisord.conf && \
    echo "command=cron-manager" >> /etc/supervisord.conf && \
    echo "autostart=true" >> /etc/supervisord.conf && \
    echo "autorestart=true" >> /etc/supervisord.conf && \
    echo "stdout_logfile=/dev/stdout" >> /etc/supervisord.conf && \
    echo "stdout_logfile_maxbytes=0" >> /etc/supervisord.conf && \
    echo "stderr_logfile=/dev/stderr" >> /etc/supervisord.conf && \
    echo "stderr_logfile_maxbytes=0" >> /etc/supervisord.conf

# Expose port for cron manager
EXPOSE 8008

# Start supervisor to run both services
CMD ["supervisord", "-c", "/etc/supervisord.conf"]

