#!/bin/bash
set -ex

pushd ../../tools/c37ingress
go build -v
ver=$(./c37ingress -version)
popd
cp ../../tools/c37ingress/c37ingress .
docker build -t btrdb/dev-c37ingress:${ver} .
docker push btrdb/dev-c37ingress:${ver}
docker tag btrdb/dev-c37ingress:${ver} btrdb/dev-c37ingress:latest
docker push btrdb/dev-c37ingress:latest
