#!/bin/bash
set -ex
docker build --no-cache -t btrdb/kubernetes-controller-manager-rbd:1.10.0 .
docker push btrdb/kubernetes-controller-manager-rbd:1.10.0
