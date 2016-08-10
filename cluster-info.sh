# This file contains the information about your servers.
# this script is only supported if you do not change this
# information half way through

# clusterinfo is a space separated set of tuples of
# nodename,public_ip,internal_ip
CLUSTER_INFO="n1,52.40.191.214,172.30.0.56 "
CLUSTER_INFO+="n2,52.10.14.135,172.30.0.57 "
CLUSTER_INFO+="n3,52.25.75.35,172.30.0.58"
export CLUSTER_INFO
export IDENTITY=$HOME/.ssh/btrdb-qs.pem
export TUNNEL_IP=52.40.191.214
export SSH_USER=ubuntu
export DOMAIN=testqs3.cal-sdb.org
eval "$(ssh-agent -s)" 2>&1 >/dev/null
ssh-add $IDENTITY
