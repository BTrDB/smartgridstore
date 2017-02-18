#!/bin/bash
set -ex

pushd ../../tools/admincliserver
go build -v
ver=$(./admincliserver -version)
popd
cp ../../tools/admincliserver/admincliserver .
docker build -t btrdb/dev-console:${ver} .
docker push btrdb/dev-console:${ver}
docker tag btrdb/dev-console:${ver} btrdb/dev-console:latest
docker push btrdb/dev-console:latest
