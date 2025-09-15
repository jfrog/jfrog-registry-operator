#!/bin/bash

# Simple build script for Linux (WSL)
echo "Building JFrog Registry Operator for Linux..."

# Create bin directory if it doesn't exist
mkdir -p bin

# Build for Linux
echo "Building for Linux x64..."
GOOS=linux GOARCH=amd64 go build -o bin/operator .

echo "Build complete! Binary is at: bin/operator"
ls -la bin/


# publish

docker build -f Dockerfile.simple -t jfrog-registry-operator:fixed1 .
docker tag jfrog-registry-operator:fixed1 metimike/jfrog-registry-operator:0.0.19
docker push metimike/jfrog-registry-operator:0.0.19