#!/bin/bash
set -e

docker build  -t immesys/btrdb-qs-wb .
docker push immesys/btrdb-qs-wb
