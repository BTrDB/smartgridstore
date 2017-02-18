#!/bin/bash
set -ex

pushd ../../tools/receiver
go build -v
ver=$(./receiver -version)
popd
cp ../../tools/receiver/receiver .
docker build -t btrdb/dev-receiver:${ver} .
docker push btrdb/dev-receiver:${ver}
docker tag btrdb/dev-receiver:${ver} btrdb/dev-receiver:latest
docker push btrdb/dev-receiver:latest
