#!/bin/bash

#set to "N" to not build
MAKE_BTRDB=n1
MAKE_MRPLOTTER=n2
MAKE_APIFRONTEND=n3
MAKE_ADMINCONSOLE=n4

USR=root
n1=node0.mask.house
n2=node1.mask.house
n3=node2.mask.house
n4=node3.mask.house
nodes=($n1 $n2 $n3 $n4)

# work out what the local versions are
BTRDB_COMMIT=$(cd $GOPATH/src/github.com/BTrDB/btrdb-server && git rev-parse HEAD)
BTRDBLIB_COMMIT=$(cd $GOPATH/src/github.com/BTrDB/btrdb && git rev-parse HEAD)
SGS_COMMIT=$(cd $GOPATH/src/github.com/BTrDB/smartgridstore && git rev-parse HEAD)
MRP_COMMIT=$(cd $GOPATH/src/github.com/BTrDB/mr-plotter && git rev-parse HEAD)

set -o pipefail

# set up the build environment on the remote machines
# run processes and store pids in array
unset pids
for idx in "${!nodes[@]}"
do
  node=${nodes[$idx]}
  ssh $USR@$node bash <<'EOF' 2>&1 | sed "s/^/[$node] /" &
set -e
rm -rf /usb/staging
mkdir -p /usb/staging/gopath/src/github.com/BTrDB
cd /usb/staging/gopath/src/github.com/BTrDB
git clone https://github.com/BTrDB/btrdb-server.git
git clone https://github.com/BTrDB/mr-plotter.git
git clone https://github.com/BTrDB/smartgridstore.git
git clone https://github.com/BTrDB/btrdb.git
EOF
  pids[idx]=$!
done

# wait for all pids
for pid in ${pids[@]}; do
    wait $pid
    if [[ $? != 0 ]]
    then
      echo "subprocess failure"
      exit 1
    fi
done

unset pids

if [[ "$MAKE_BTRDB" != "N" ]]
then
ssh $USR@${!MAKE_BTRDB} bash <<'EOF' 2>&1 | sed "s/^/[btrdbd] /" &
set -e
export GOPATH=/usb/staging/gopath
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:/usb/staging/gopath/bin
go get github.com/golang/dep/cmd/dep
go get github.com/maruel/panicparse
cd $GOPATH/src/github.com/BTrDB/btrdb-server
dep ensure -v
cd btrdbd
go build -v
BTRDB_VER=`./btrdbd -version`
cd ../k8scontainer2
cp ../btrdbd/btrdbd .
docker build --no-cache -t btrdb/arm-db:$BTRDB_VER .
echo "btrdb container build complete"
docker push btrdb/arm-db:$BTRDB_VER
EOF
pids[0]=$!
fi

set -x

if [[ "$MAKE_MRPLOTTER" != "N" ]]
then
ssh $USR@${!MAKE_MRPLOTTER} bash <<'EOF' 2>&1 | sed "s/^/[mplotter] /" &
set -e
export GOPATH=/usb/staging/gopath
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:/usb/staging/gopath/bin
go get -u github.com/golang/dep/cmd/dep
cd $GOPATH/src/github.com/BTrDB/mr-plotter
dep ensure -v
go build -v
mrp_ver=`./mr-plotter -version`
cd tools/hardcodecert
go build
cd ../setsessionkeys
go build
cd ../..
cd container
cp ../mr-plotter .
cp ../tools/hardcodecert/hardcodecert .
cp ../tools/setsessionkeys/setsessionkeys .
docker build --no-cache -t btrdb/arm-mrplotter:$mrp_ver .
echo "mrplotter container build complete"
docker push btrdb/arm-mrplotter:$mrp_ver
EOF
pids[1]=$!
fi

if [[ "$MAKE_APIFRONTEND" != "N" ]]
then
ssh $USR@${!MAKE_APIFRONTEND} bash <<'EOF' 2>&1 | sed "s/^/[apifrontend] /" &
set -e
export GOPATH=/usb/staging/gopath
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:/usb/staging/gopath/bin
go get -u github.com/golang/dep/cmd/dep
cd $GOPATH/src/github.com/BTrDB/smartgridstore
go get -u github.com/jteeuwen/go-bindata/go-bindata
dep ensure -v

pushd tools/apifrontend
go generate -v
go build -v
ver=$(./apifrontend -version)
popd
cp tools/apifrontend/apifrontend containers/apifrontend/
cd containers/apifrontend
docker build --no-cache -t btrdb/arm-apifrontend:${ver} .
echo "apifrontend container build complete"
docker push btrdb/arm-apifrontend:${ver}
EOF
pids[2]=$!
fi

if [[ "$MAKE_ADMINCONSOLE" != "N" ]]
then
ssh $USR@${!MAKE_ADMINCONSOLE} bash <<'EOF' 2>&1 | sed "s/^/[adminconsole] /" &
set -e
export GOPATH=/usb/staging/gopath
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:/usb/staging/gopath/bin
go get -u github.com/golang/dep/cmd/dep
cd $GOPATH/src/github.com/BTrDB/smartgridstore
dep ensure -v

pushd tools/admincliserver
go generate -v
go build -v
ver=$(./admincliserver -version)
popd
cp tools/admincliserver/admincliserver containers/adminconsole/
cd containers/adminconsole
docker build --no-cache -t btrdb/arm-console:${ver} .
echo "admin console container build complete"
docker push btrdb/arm-console:${ver}
EOF
pids[3]=$!
fi

# wait for all pids
for pid in ${pids[@]}; do
    wait $pid
    if [[ $? != 0 ]]
    then
      echo "subprocess failure"
    fi
done

echo "DONE!"
