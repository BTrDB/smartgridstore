#!/bin/bash
set -ex

pushd ../../tools/ingester
go build -v
ver=$(./ingester -version)
popd
cp ../../tools/ingester/ingester .
docker build -t btrdb/dev-ingester:${ver} .
docker push btrdb/dev-ingester:${ver}
docker tag btrdb/dev-ingester:${ver} btrdb/dev-ingester:latest
docker push btrdb/dev-ingester:latest
