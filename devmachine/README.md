# Running Locally

This will set up a local environment for development under Linux or Mac.

1. Install [Docker](https://www.docker.com/)
2. Set your desired `DEVMACHINE_BASE` in [environment.sh](environment.sh)
3. Ensure your hard drive has ~20GB of free space and is using ext4
4. Run the following:

    ```
    source environment.sh
    sudo -E ./start_devmachine.sh
    ```

5. You can verify Docker environment is running with `docker ps`:

    ```
    $ docker ps
    CONTAINER ID        IMAGE                     COMMAND                 CREATED             STATUS              PORTS                                            NAMES
    6087d3c575e2        btrdb/apifrontend:4.7.0   "/bin/apifrontend"      12 minutes ago      Up 12 minutes       0.0.0.0:4410->4410/tcp, 0.0.0.0:9000->9000/tcp   devmachine-apifrontend
    0a1b3e9f12f8        btrdb/mrplotter:4.7.0     "/entrypoint.sh"        12 minutes ago      Up 12 minutes       0.0.0.0:8888->443/tcp                            devmachine-mrplotter
    035e68238adc        btrdb/console:4.7.0       "/bin/admincliserver"   13 minutes ago      Up 13 minutes       0.0.0.0:2222->2222/tcp                           devmachine-console
    f5430854a235        btrdb/db:4.7.0            "/entrypoint.sh"        13 minutes ago      Up 13 minutes                                                        devmachine-btrdbd
    cc203a829bdc        btrdb/stubetcd:latest     "/bin/etcd"             13 minutes ago      Up 13 minutes                                                        devmachine-etcd
    84357a7d71e9        btrdb/cephdaemon          "/entrypoint.sh osd"    13 minutes ago      Up 13 minutes                                                        devmachine-ceph-osd-3
    ceae99ebf8b1        btrdb/cephdaemon          "/entrypoint.sh osd"    13 minutes ago      Up 13 minutes                                                        devmachine-ceph-osd-2
    305c36b9c162        btrdb/cephdaemon          "/entrypoint.sh osd"    14 minutes ago      Up 13 minutes                                                        devmachine-ceph-osd-1
    6cac4bbf7d37        btrdb/cephdaemon          "/entrypoint.sh osd"    14 minutes ago      Up 14 minutes                                                        devmachine-ceph-osd-0
    08b1d8247399        btrdb/cephdaemon          "/entrypoint.sh mgr"    14 minutes ago      Up 14 minutes                                                        devmachine-ceph-mgr
    5258556ca408        btrdb/cephdaemon          "/entrypoint.sh mon"    14 minutes ago      Up 14 minutes                                                        devmachine-ceph-mon
    ```

6. You can stop your dev machine with `sudo -E ./teardown_devmachine` (make sure you still have `environment.sh` sourced)
