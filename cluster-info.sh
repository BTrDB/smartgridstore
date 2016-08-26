# This file contains the information about your servers.
# The tools assume that you only modify this file
# once, at the beginning, so double check it for correctness.

# clusterinfo is a space separated set of tuples of
# nodename,public_ip,private_ip
CLUSTER_INFO="node0,128.32.3.10,10.0.0.10 "
CLUSTER_INFO+="node1,128.32.3.11,10.0.0.11 "
CLUSTER_INFO+="node2,128.32.3.12,10.0.0.12 "
export CLUSTER_INFO

# This is the ssh key file that enables login. If you used
# EC2, this will likely end in .pem
export IDENTITY=$HOME/.ssh/id_rsa

# This can be any of the above three nodes. It is the machine
# that will be connected to when controlling the cluster
export TUNNEL_IP=128.32.3.10

# This is the username to use. It must be able to log in without
# a password using $IDENTITY above, and must be able to do passwordless
# sudo (default on EC2)
export SSH_USER=ubuntu

# This is the domain name you want to set up for your master server
# you can leave it as-is if you are not setting up the plotter
export DOMAIN=my.plotter.domain

# This is required for some of the tools
chmod 0600 $IDENTITY
eval "$(ssh-agent -s)" 2>&1 >/dev/null
ssh-add $IDENTITY
