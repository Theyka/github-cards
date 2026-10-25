# syntax=docker/dockerfile:1

# --- Build stage -------------------------------------------------------------
FROM golang:1.27-alpine AS build

WORKDIR /src

# The service only depends on the standard library, so no go.sum is needed.
COPY go.mod ./
COPY main.go ./
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/github-cards .

# --- Runtime stage -----------------------------------------------------------
FROM alpine:3.20

# ca-certificates: HTTPS calls to api.github.com.
# tzdata: the streak card's timezone parameter.
# curl: Coolify's default health check command (wget is used by the Dockerfile HEALTHCHECK).
RUN apk add --no-cache ca-certificates tzdata curl

COPY --from=build /out/github-cards /usr/local/bin/github-cards

ENV PORT=3000
EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s \
  CMD wget -qO- "http://127.0.0.1:${PORT}/healthz" || exit 1

ENTRYPOINT ["github-cards"]
