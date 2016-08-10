#!/bin/bash
set -e
docker build --no-cache -t immesys/btrdb-qs-sync .
docker push immesys/btrdb-qs-sync
