#!/bin/bash
set -e
go build github.com/immesys/smartgridstore/tools/receiver
docker build -t btrdb/sgs/receiver .
docker push btrdb/sgs/receiver
