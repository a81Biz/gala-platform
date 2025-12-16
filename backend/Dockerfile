# backend/Dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build API
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api
# Build Worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates && update-ca-certificates

COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker

# Por defecto corre API; el worker en compose usa command: ["./worker"]
EXPOSE 8080
CMD ["./api"]
