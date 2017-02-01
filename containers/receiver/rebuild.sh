#!/bin/bash
set -ex

pushd ../../tools/receiver
go build -v
ver=$(./receiver -version)
popd
cp ../../tools/receiver/receiver .
docker build -t btrdb/receiver:${ver} .
docker push btrdb/receiver:${ver}
docker tag btrdb/receiver:${ver} btrdb/receiver:latest
docker push btrdb/receiver:latest
