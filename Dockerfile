# Base image Go 1.25.3 Alpine
FROM golang:1.25.3-alpine

# Install git & bash (optional, untuk go modules)
RUN apk add --no-cache git bash

# Working directory
WORKDIR /app

# Copy go.mod & go.sum, download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build executable
RUN go build -o app .

# Expose port Fiber backend
EXPOSE 9000

# Run binary
CMD ["./app"]
