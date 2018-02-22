#!/bin/bash
set -ex
docker build --no-cache -t btrdb/kubernetes-controller-manager-rbd:1.9.3 .
docker push btrdb/kubernetes-controller-manager-rbd:1.9.3
