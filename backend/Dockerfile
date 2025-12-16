# =====================
# Build stage
# =====================
FROM golang:1.24-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build API
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /out/api ./cmd/api

# Build WORKER
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /out/worker ./cmd/worker


# =====================
# Runtime: API
# =====================
FROM alpine:3.20 AS api
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/api /app/api
EXPOSE 8080
CMD ["/app/api"]


# =====================
# Runtime: WORKER
# =====================
FROM alpine:3.20 AS worker
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/worker /app/worker
CMD ["/app/worker"]
