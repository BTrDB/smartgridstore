#!/bin/bash
docker build --no-cache -t btrdb/stubetcd:3.1.10 .
docker push btrdb/stubetcd:3.1.10
