FROM golang:1.24-alpine AS builder
WORKDIR /app/admin-api
COPY pkg/middleware /app/pkg/middleware
COPY admin-api/go.mod admin-api/go.sum* ./
RUN go mod download 2>/dev/null || true
COPY admin-api/ .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /admin-api ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /admin-api .
USER nobody
EXPOSE 8081
CMD ["./admin-api"]
