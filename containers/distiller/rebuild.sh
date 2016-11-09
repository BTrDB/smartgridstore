#!/bin/bash
set -e
docker build -t btrdb/distiller .
docker push btrdb/distiller
