# source this script

# which version of BTrDB and tools to install
export VERSION=4.6.5
# where to create the 4 osd directories to store data
export OSDBASE=/srv/btrdbdev/ceph
# where to put the etcd data
export ETCDBASE=/srv/btrdbdev/etcd
# which docker network to use (will be created if it doesn't exist)
export DOCKERNET=cephnet
# subnet .24 prefix to use
export SUB24=172.29.0
# container name prefix
export CONTAINER_PREFIX=btrdbdev-

export HOTPOOL=btrdbhot
export COLDPOOL=btrdbcold

# put the dv-ceph command in the path
# this only works if you source this from the directory its actually in
export PATH=$PATH:$PWD/bin
