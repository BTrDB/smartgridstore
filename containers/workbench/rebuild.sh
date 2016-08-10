#!/bin/bash
set -e
docker build --no-cache -t immesys/btrdb-qs-wb .
docker push immesys/btrdb-qs-wb
