# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY cmd/ ./cmd/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v3 \
    go build \
    -ldflags="-s -w" \
    -trimpath \
    -o server \
    ./cmd/server

# ── Stage 2: runtime ────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

WORKDIR /app

COPY --from=builder --chown=65532:65532 /app/server ./server
COPY --chown=65532:65532 resources/ ./resources/

# Defaults — podem ser sobrescritos no docker-compose.
# GOMAXPROCS=1   → 0.45 CPU não comporta múltiplos Ps sem thrash.
# GOGC=300       → menos ciclos de GC (heap base ~6.4MB).
# GOMEMLIMIT     → orienta o GC sem pegar OOM (limite do container é 160MB).
ENV GOMAXPROCS=1 \
    GOGC=300 \
    GOMEMLIMIT=120MiB

EXPOSE 3000

CMD ["/app/server"]
