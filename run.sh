#!/usr/bin/env bash

if  [ -z "$1" ] || [ "$1" =  "-h" ] || [ "$1" = "--help" ]; then
  echo "
Usage: $0 CONTENT_ROOT

Start the file server.
"
  exit 1
fi

CONTENT_ROOT=$1
IMAGE_NAME="file-server:$(git rev-parse HEAD)"
CONTAINER_NAME="file-server-${RANDOM}"

docker build . -t "$IMAGE_NAME"
docker run --rm \
  --publish 8080:8080 \
  --name "$CONTAINER_NAME" \
  --volume "${CONTENT_ROOT}:/srv" \
  --env "FILE_SERVER_LISTEN_ADDRESS=0.0.0.0:8080" \
  --env "FILE_SERVER_CONTENT_ROOT=/srv" \
  "$IMAGE_NAME"
