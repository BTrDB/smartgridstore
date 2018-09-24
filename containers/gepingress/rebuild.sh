#!/bin/bash
set -ex
pushd ../../tools/gepingress
go build -v
ver=$(./gepingress -version)
popd
cp ../../tools/gepingress/gepingress .
docker build -t btrdb/dev-gepingress:${ver} .
docker push btrdb/dev-gepingress:${ver}
docker tag btrdb/dev-gepingress:${ver} btrdb/dev-gepingress:latest
docker push btrdb/dev-gepingress:latest
