#!/bin/bash

echo "determining version"

set -ex

PFX=""

pushd $GOPATH/src/github.com/BTrDB/btrdb-server/btrdbd
go build
btrdb_ver=`./btrdbd -version`
echo "BTrDB version is $btrdb_ver"

target_ver=$btrdb_ver
popd

pushd $GOPATH/src/github.com/BTrDB/mr-plotter
go get -u ./...
go build
mrp_ver=`./mr-plotter -version`
if [[ "$mrp_ver" != "$target_ver" ]]
then
  echo "Mr. Plotter version mismatch - got $mrp_ver"
  exit 1
fi
pushd tools/hardcodecert
go build -v
popd
pushd tools/setsessionkeys
go build -v
popd

popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/tools/admincliserver
go get -u ./...
go build
cli_ver=`./admincliserver -version`
if [[ "$cli_ver" != "$target_ver" ]]
then
  echo "AdminCLI version mismatch - got $cli_ver"
  exit 1
fi
popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/tools/ingester
go get -u ./...
go build
ing_ver=`./ingester -version`
if [[ "$ing_ver" != "$target_ver" ]]
then
  echo "Ingester version mismatch - got $ing_ver"
  exit 1
fi
popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/tools/receiver
go get -u ./...
go build
rec_ver=`./receiver -version`
if [[ "$rec_ver" != "$target_ver" ]]
then
  echo "Receiver version mismatch - got $rec_ver"
  exit 1
fi
popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/tools/pmu2btrdb
go get -u ./...
go build
pmu2btrdb_ver=`./pmu2btrdb -version`
if [[ "$pmu2btrdb_ver" != "$target_ver" ]]
then
  echo "PMU2BTrDB version mismatch - got $pmu2btrdb_ver"
  exit 1
fi
popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/tools/simulator
go get -u ./...
go build
simulator_ver=`./simulator -version`
if [[ "$simulator_ver" != "$target_ver" ]]
then
  echo "Simulator version mismatch - got $simulator_ver"
  exit 1
fi
popd

echo "All versions match $target_ver, building containers"
set -e

pushd $GOPATH/src/github.com/BTrDB/btrdb-server/k8scontainer
cp ../btrdbd/btrdbd .
docker build -t btrdb/${PFX}db:$target_ver .
popd

pushd $GOPATH/src/github.com/BTrDB/mr-plotter/container
cp ../mr-plotter .
cp ../tools/hardcodecert/hardcodecert .
cp ../tools/setsessionkeys/setsessionkeys .
docker build --no-cache -t btrdb/${PFX}mrplotter:$target_ver .
popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/containers/adminconsole
cp ../../tools/admincliserver/admincliserver .
docker build -t  btrdb/${PFX}console:$target_ver .
popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/containers/ingester
cp ../../tools/ingester/ingester .
docker build -t btrdb/${PFX}ingester:$target_ver .
popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/containers/c37ingress
cp ../../tools/c37ingress/c37ingress .
docker build -t btrdb/${PFX}c37ingress:$target_ver .
popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/containers/receiver
cp ../../tools/receiver/receiver .
docker build -t btrdb/${PFX}receiver:$target_ver .
popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/containers/pmu2btrdb
cp ../../tools/pmu2btrdb/pmu2btrdb .
docker build -t btrdb/${PFX}pmu2btrdb:$target_ver .
popd

pushd $GOPATH/src/github.com/BTrDB/smartgridstore/containers/simulator
cp ../../tools/simulator/simulator .
docker build -t btrdb/${PFX}simulator:$target_ver .
popd

echo "All containers built ok for $PFX-$target_ver , pushing containers"

docker push btrdb/${PFX}mrplotter:$target_ver
docker push btrdb/${PFX}console:$target_ver
docker push btrdb/${PFX}ingester:$target_ver
docker push btrdb/${PFX}c37ingress:$target_ver
docker push btrdb/${PFX}receiver:$target_ver
docker push btrdb/${PFX}pmu2btrdb:$target_ver
docker push btrdb/${PFX}simulator:$target_ver
docker push btrdb/${PFX}db:$target_ver

echo "DONE!"
echo "Release ${PFX}$target_ver is published"
