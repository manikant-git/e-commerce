# Build stage
FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go clean -modcache && go mod tidy && go mod download
COPY src/ ./src/
RUN CGO_ENABLED=0 GOOS=linux go build -o notification-service ./src/main.go

# Runtime stage
FROM alpine:3.18
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=build /app/notification-service .

USER appuser

EXPOSE 9000

CMD ["./notification-service"]
