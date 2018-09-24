#!/bin/bash

set -ex

# compile boost
wget https://dl.bintray.com/boostorg/release/1.68.0/source/boost_1_68_0.tar.gz
tar -xvf boost_1_68_0.tar.gz
mv boost_1_68_0 boost
cd boost
./bootstrap.sh
./b2 -j 12
cd /
export CPATH=/boost

# compile gsf libs
git clone https://github.com/GridProtectionAlliance/gsf.git
mkdir output
cd output
cmake -DCMAKE_BUILD_TYPE=Debug /gsf/Source/Libraries/TimeSeriesPlatformLibrary
make

# export results
cp -r Include /build
cp -r Libraries /build
cp -r /boost/stage/lib /build/Libraries/boost
cp -r /boost/boost /build/Include
echo "Done!"
