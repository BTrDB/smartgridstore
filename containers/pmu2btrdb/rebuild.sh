#!/bin/bash
set -ex

pushd ../../tools/pmu2btrdb
go build -v
ver=$(./pmu2btrdb -version)
popd
cp ../../tools/pmu2btrdb/pmu2btrdb .
docker build -t btrdb/pmu2btrdb:${ver} .
docker push btrdb/pmu2btrdb:${ver}
docker tag btrdb/pmu2btrdb:${ver} btrdb/pmu2btrdb:latest
docker push btrdb/pmu2btrdb:latest
