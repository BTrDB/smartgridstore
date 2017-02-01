#!/bin/bash
set -e
docker build --no-cache -t btrdb/btrdbd:4
docker push btrdb/btrdbd:4
