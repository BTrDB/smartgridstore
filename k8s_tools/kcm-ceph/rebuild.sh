#!/bin/bash
set -ex
docker build -t btrdb/kubernetes-controller-manager-rbd:1.6.1 .
docker push btrdb/kubernetes-controller-manager-rbd:1.6.1
