#!/bin/bash
set -e
docker build --no-cache -t immesys/mrplotter .
docker push immesys/mrplotter
