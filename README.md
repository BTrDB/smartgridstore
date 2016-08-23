# btrdb-upmu-quickstart
Let us build a cluster for you


## Overview

Configure the `cluster-info.sh` file with the information for your cluster.

Then run `bin/qs-prepare-servers.sh` to update the servers and install
docker / fleet. You may need to run it twice if the updates required a reboot.
It will tell you.

Then run `bin/qs-generate-config.sh` to generate a cluster configuration.

You should edit the configuration, following the instructions, and then run 
`bin/qs-execute-config.sh`.

There used to be better instructions but I lost them. I'll come back
and improve this in the next few wooks.
