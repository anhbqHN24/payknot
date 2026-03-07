FROM golang:1.24.2-alpine AS builder
WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/backend ./main.go

FROM alpine:3.20 AS runner
WORKDIR /app
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 appuser

COPY --from=builder /bin/backend /app/backend
USER appuser
EXPOSE 8080
CMD ["/app/backend"]
