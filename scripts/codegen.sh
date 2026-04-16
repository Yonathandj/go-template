#!/bin/bash

set -e

CONFIG="api/server/config.yaml"
SPECIFICATIONS="api/server/specifications"
OUTPUT_DIR="internal/api/server/gen"

mkdir -p "$OUTPUT_DIR"

for file in "$SPECIFICATIONS"/*.yaml; do
  name=$(basename "$file" .yaml)

  echo "→ Generating $name"

  go tool oapi-codegen \
    -config "$CONFIG" \
    -o "$OUTPUT_DIR/$name.gen.go" \
    "$file"
done