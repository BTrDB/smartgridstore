# source this script

# which version of BTrDB and tools to install
export VERSION=4.7.0
# where to create the 4 osd directories to store data
export OSDBASE=/srv/devmachine/ceph
# where to put the etcd data
export ETCDBASE=/srv/devmachine/etcd
# which docker network to use (will be created if it doesn't exist)
export DOCKERNET=cephnet
# subnet .24 prefix to use
export SUB24=172.29.0
# container name prefix
export CONTAINER_PREFIX=devmachine-

export HOTPOOL=btrdbhot
export COLDPOOL=btrdbcold

# the port on the host to bind the plotter to
export PLOTTER_PORT=8888
# the port on the host to bind the BTrDB API to
export API_GRPC_PORT=4410
# the port on the host to bind the HTTP BTrDB API to
export API_HTTP_PORT=9000

# the port to bind the admin console SSH to
export CONSOLE_PORT=2222

# put the dv-ceph command in the path
# this only works if you source this from the directory its actually in
export PATH=$PATH:$PWD/bin
