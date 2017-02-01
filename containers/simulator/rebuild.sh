#!/bin/bash
set -ex

pushd ../../tools/simulator
go build -v
ver=$(./simulator -version)
popd
cp ../../tools/simulator/simulator .
docker build -t btrdb/simulator:${ver} .
docker push btrdb/simulator:${ver}
docker tag btrdb/simulator:${ver} btrdb/simulator:latest
docker push btrdb/simulator:latest
