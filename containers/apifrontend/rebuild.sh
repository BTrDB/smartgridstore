#!/bin/bash
set -ex

pushd ../../tools/apifrontend
go build -v
ver=$(./apifrontend -version)
popd
cp ../../tools/apifrontend/apifrontend .
docker build -t btrdb/dev-apifrontend:${ver} .
docker push btrdb/dev-apifrontend:${ver}
docker tag btrdb/dev-apifrontend:${ver} btrdb/dev-apifrontend:latest
docker push btrdb/dev-apifrontend:latest
