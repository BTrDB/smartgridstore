#!/bin/bash
set -ex

pushd ../../tools/certproxy/
go build
ver=$(./certproxy -version)
cp certproxy ../../containers/certproxy/
popd

docker build -t btrdb/dev-certproxy:${ver} .
docker push btrdb/dev-certproxy:${ver}
