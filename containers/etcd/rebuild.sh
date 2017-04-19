#!/bin/bash
docker build -t btrdb/etcd:3.1.5 .
docker push btrdb/etcd:3.1.5
