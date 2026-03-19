# Stage 1: Build Backend
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/label-station .

# Stage 2: Runner
FROM alpine:latest
WORKDIR /app
# Copy backend binary
COPY --from=builder /app/label-station .

EXPOSE 8080

ENTRYPOINT ["./label-station"]

