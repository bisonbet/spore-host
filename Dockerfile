# Multi-stage build — produces a minimal image with all spore-host CLI tools.
# Contains: truffle, spawn, lagotto, spore-host-mcp
# Excludes: spored (daemon that runs ON instances, not in containers)

FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY . .

# Build all four binaries statically
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/truffle ./truffle && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/spawn ./spawn && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/lagotto ./lagotto && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/spore-host-mcp ./mcp

# Minimal runtime image
FROM alpine:3.21

# Runtime deps: ca-certs (HTTPS), openssh-client (spawn connect), bash
RUN apk add --no-cache ca-certificates openssh-client bash

COPY --from=builder /out/truffle /usr/local/bin/truffle
COPY --from=builder /out/spawn /usr/local/bin/spawn
COPY --from=builder /out/lagotto /usr/local/bin/lagotto
COPY --from=builder /out/spore-host-mcp /usr/local/bin/spore-host-mcp

# Default AWS credential chain works via env vars or mounted ~/.aws
ENV AWS_SDK_LOAD_CONFIG=1

LABEL org.opencontainers.image.source="https://github.com/spore-host/spore-host" \
      org.opencontainers.image.description="spore-host CLI tools: truffle, spawn, lagotto, spore-host-mcp" \
      org.opencontainers.image.licenses="Apache-2.0"

CMD ["spawn", "--help"]
