#!/bin/bash
docker build --no-cache -t btrdb/stubetcd:latest .
docker push btrdb/stubetcd:latest
