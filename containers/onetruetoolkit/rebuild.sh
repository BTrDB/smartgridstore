#!/bin/bash
set -ex

docker build -t btrdb/ottk .
docker push btrdb/ottk .
