# Stage 1: Build Frontend
FROM node:18-alpine AS fe-builder
WORKDIR /app/fe
COPY fe/package*.json ./
RUN npm install
COPY fe/ .
RUN npm run build

# Stage 2: Build Backend
FROM golang:1.24-alpine AS be-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# We need to preserve the built frontend files OR let Go pick them up.
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/label-station main.go

# Stage 3: Runner
FROM alpine:latest
WORKDIR /app
# Copy backend binary
COPY --from=be-builder /app/label-station .
# Copy frontend static files to the expected directory
COPY --from=fe-builder /app/fe/dist ./fe/dist

EXPOSE 8080

ENTRYPOINT ["./label-station"]
