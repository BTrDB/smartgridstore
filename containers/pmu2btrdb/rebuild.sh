#!/bin/bash
set -ex

pushd ../../tools/pmu2btrdb
go build -v
ver=$(./pmu2btrdb -version)
popd
cp ../../tools/pmu2btrdb/pmu2btrdb .
docker build -t btrdb/dev-pmu2btrdb:${ver} .
docker push btrdb/dev-pmu2btrdb:${ver}
docker tag btrdb/dev-pmu2btrdb:${ver} btrdb/dev-pmu2btrdb:latest
docker push btrdb/dev-pmu2btrdb:latest
