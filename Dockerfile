# Step 1: Use the official Golang image as the build environment.
# This image includes all the tools needed to compile Go applications.
FROM golang:1.21 as builder

# Version will be passed as a part of the build.
ARG VERSION=dev

# Set the Current Working Directory inside the container.
WORKDIR /app

# Copy the local package files to the container's workspace.
COPY . .

# Build the Go app for a Linux system. The -o flag specifies the output binary
# name.
RUN VERSION=${VERSION} make build

# Step 2: Use a Docker multi-stage build to create a lean production image.
# Start from a scratch (empty) image to keep the image size small.
FROM scratch

# Copy the binary from the builder stage to the production image.
COPY --from=builder /app/udptlspipe /

# Exact protocol depends on operation mode: tcp for server and udp for client.
EXPOSE 8443/tcp
EXPOSE 8443/udp

ENTRYPOINT ["/udptlspipe", "-l", "127.0.0.1:8443"]