#!/bin/bash

rm -rf build
mkdir build
docker run -v $PWD/build:/build -it gep_lib_build
