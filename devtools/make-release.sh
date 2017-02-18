#!/bin/bash

echo "determining version"

set -e

pushd $GOPATH/src/github.com/SoftwareDefinedBuildings/btrdb/btrdbd
go build
btrdb_ver=`./btrdbd -version`
echo "BTrDB version is $btrdb_ver"
target_ver=$btrdb_ver
popd

pushd $GOPATH/src/github.com/SoftwareDefinedBuildings/mr-plotter
go get -u ./...
go build
mrp_ver=`./mr-plotter -version`
if [[ "$mrp_ver" != "$target_ver" ]]
then
  echo "Mr. Plotter version mismatch - got $mrp_ver"
  exit 1
fi
pushd ../tools/hardcodecert
go build -v
popd
pushd ../tools/setsessionkeys
go build -v
popd

popd

pushd $GOPATH/src/github.com/immesys/smartgridstore/tools/admincliserver
go get -u ./...
go build
cli_ver=`./admincliserver -version`
if [[ "$cli_ver" != "$target_ver" ]]
then
  echo "AdminCLI version mismatch - got $cli_ver"
  exit 1
fi
popd

pushd $GOPATH/src/github.com/immesys/smartgridstore/tools/ingester
go get -u ./...
go build
ing_ver=`./ingester -version`
if [[ "$ing_ver" != "$target_ver" ]]
then
  echo "Ingester version mismatch - got $ing_ver"
  exit 1
fi
popd

pushd $GOPATH/src/github.com/immesys/smartgridstore/tools/receiver
go get -u ./...
go build
rec_ver=`./receiver -version`
if [[ "$rec_ver" != "$target_ver" ]]
then
  echo "Receiver version mismatch - got $rec_ver"
  exit 1
fi
popd

pushd $GOPATH/src/github.com/immesys/smartgridstore/tools/pmu2btrdb
go get -u ./...
go build
pmu2btrdb_ver=`./pmu2btrdb -version`
if [[ "$pmu2btrdb_ver" != "$target_ver" ]]
then
  echo "PMU2BTrDB version mismatch - got $pmu2btrdb_ver"
  exit 1
fi
popd

pushd $GOPATH/src/github.com/immesys/smartgridstore/tools/simulator
go get -u ./...
go build
simulator_ver=`./simulator -version`
if [[ "$simulator_ver" != "$target_ver" ]]
then
  echo "Simulator version mismatch - got $simulator_ver"
  exit 1
fi
popd

echo "All versions match $target_ver, building and pushing containers"

set -e

pushd $GOPATH/src/github.com/SoftwareDefinedBuildings/btrdb/k8s_container
cp ../btrdbd/btrdbd .
docker build btrdb/db:$target_ver .
docker push btrdb/db:$target_ver
popd

pushd $GOPATH/src/github.com/SoftwareDefinedBuildings/mr-plotter/container
cp ../mr-plotter .
cp ../tools/hardcodecert/hardcodecert .
cp ../tools/setsessionkeys/setsessionkeys .
docker build btrdb/mrplotter:$target_ver .
docker push btrdb/mrplotter:$target_ver
popd

pushd $GOPATH/src/github.com/immesys/smartgridstore/containers/adminconsole
cp ../../tools/admincliserver/admincliserver .
docker build btrdb/console:$target_ver .
docker push btrdb/console:$target_ver
popd

pushd $GOPATH/src/github.com/immesys/smartgridstore/containers/ingester
cp ../../tools/ingester/ingester .
docker build btrdb/ingester:$target_ver .
docker push btrdb/ingester:$target_ver
popd

pushd $GOPATH/src/github.com/immesys/smartgridstore/containers/receiver
cp ../../tools/receiver/receiver .
docker build btrdb/receiver:$target_ver .
docker push btrdb/receiver:$target_ver
popd

pushd $GOPATH/src/github.com/immesys/smartgridstore/containers/pmu2btrdb
cp ../../tools/pmu2btrdb/pmu2btrdb .
docker build btrdb/pmu2btrdb:$target_ver .
docker push btrdb/pmu2btrdb:$target_ver
popd

pushd $GOPATH/src/github.com/immesys/smartgridstore/containers/simulator
cp ../../tools/simulator/simulator .
docker build btrdb/simulator:$target_ver .
docker push btrdb/simulator:$target_ver
popd

echo "DONE!"
echo "Release $target_ver is published"
