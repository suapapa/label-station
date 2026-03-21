# Stage 1: Build Backend
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /app/label-station .

# Stage 2: Runner
FROM alpine:latest
WORKDIR /app
# Copy backend binary
COPY --from=builder /app/label-station .

EXPOSE 8080

ENTRYPOINT ["./label-station"]

