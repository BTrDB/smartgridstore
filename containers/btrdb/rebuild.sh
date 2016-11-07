#!/bin/bash
set -e
docker build --no-cache -t immesys/btrdb-ceph-3.4 .
docker push immesys/btrdb-ceph-3.4
