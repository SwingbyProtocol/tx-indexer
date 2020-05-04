# Dockerfile References: https://docs.docker.com/engine/reference/builder/

# Start from the latest golang base image
FROM golang:latest as builder

# Add Maintainer Info
LABEL maintainer="Yusaku Senga <yusaku@swingby.network>"

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN make build-linux-amd64

######## Start a new stage from scratch #######
FROM gcr.io/distroless/base-debian10

WORKDIR /var/app

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/bin/tx-indexer-linux-amd64 .

# Expose port 9096 and 9099 to the outside world
EXPOSE 9096 9099

# Command to run the executable
ENTRYPOINT ["/var/app/tx-indexer-linux-amd64"]
