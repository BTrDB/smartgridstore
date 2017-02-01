#!/bin/bash
set -ex

pushd ../../tools/ingester
go build -v
ver=$(./ingester -version)
popd
cp ../../tools/ingester/ingester .
docker build -t btrdb/ingester:${ver} .
docker push btrdb/ingester:${ver}
docker tag btrdb/ingester:${ver} btrdb/ingester:latest
docker push btrdb/ingester:latest
