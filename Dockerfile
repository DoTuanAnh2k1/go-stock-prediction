FROM golang:1.23-alpine
# Install necessary packages
RUN apk add --no-cache git make
# Set the working directory
WORKDIR /app
# Copy the Go module files
COPY go.mod go.sum ./