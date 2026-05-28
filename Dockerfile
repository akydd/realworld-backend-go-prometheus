# Stage 1: Build the Go binary
FROM golang:1.26.1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server

# Stage 2: Development with hot reload via air
FROM golang:1.26.1-alpine AS dev
RUN go install github.com/air-verse/air@latest
WORKDIR /app
CMD ["air"]

# Stage 3: Run the application from a minimal base image
FROM scratch
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8090
EXPOSE 8099
CMD ["./server"]
