#!/usr/bin/env bash

set -euo pipefail

BOOTSTRAP="${1:-127.0.0.1:9092}"
TOPIC="${2:-topic-leonardo-like}"
PARTITIONS="${3:-8}"
REPLICATION="${4:-3}"

if docker exec kafka1 rpk topic list --brokers "${BOOTSTRAP}" | grep -q "^${TOPIC}[[:space:]]"; then
  echo "Topic ${TOPIC} already exists"
else
  docker exec kafka1 rpk topic create "${TOPIC}" \
    --brokers "${BOOTSTRAP}" \
    --partitions "${PARTITIONS}" \
    --replicas "${REPLICATION}"
fi

docker exec kafka1 rpk topic describe "${TOPIC}" \
  --brokers "${BOOTSTRAP}"
