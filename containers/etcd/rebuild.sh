#!/bin/bash
docker build --no-cache -t btrdb/etcd:3.1.7 .
docker push btrdb/etcd:3.1.7
