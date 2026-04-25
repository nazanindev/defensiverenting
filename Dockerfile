# syntax=docker/dockerfile:1

# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/server  ./cmd/server  && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/ingest  ./cmd/ingest

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /bin/server  /server
COPY --from=build /bin/ingest  /ingest

# web/static is served directly from the filesystem in development;
# in production the same files are embedded in the binary via embed.FS (TODO: v1.1).
# For now, copy them so the container has them at the expected path.
COPY --from=build /src/web/static /web/static

EXPOSE 8080

USER nonroot

ENTRYPOINT ["/server"]
