#!/bin/bash
set -e
docker build --no-cache -t immesys/btrdb-qs-receiver .
docker push immesys/btrdb-qs-receiver
