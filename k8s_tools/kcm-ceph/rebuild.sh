#!/bin/bash
set -ex
docker build --no-cache -t btrdb/kubernetes-controller-manager-rbd:1.8.5 .
docker push btrdb/kubernetes-controller-manager-rbd:1.8.5
