# syntax=docker/dockerfile:1
# Multi-stage build producing a static, non-root image. Copy to the repository
# root. Keep GO_VERSION aligned with go.mod and CI.
ARG GO_VERSION=1.27

FROM golang:${GO_VERSION} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
USER nonroot:nonroot
# Exec form: the process receives SIGTERM directly for graceful shutdown.
ENTRYPOINT ["/server"]
