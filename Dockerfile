FROM golang:1.21-alpine AS build

# Build arg to select which app to build, e.g. hermes, atlas, zeus
ARG APP_NAME

WORKDIR /src

# Pre-cache modules separately for faster rebuilds
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy only necessary files for building
# lib/ contains shared packages imported by all apps
COPY lib/ ./lib/

# Copy the specific app being built
COPY apps/${APP_NAME}/ ./apps/${APP_NAME}/

# Build the selected app with module and build cache
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /out/app ./apps/${APP_NAME}

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]


