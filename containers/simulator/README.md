# PMU simulator

Example invocation:

docker run -it \
 -e SIMULATOR_TARGET='foo.com:1883' \
 -e SIMULATOR_SERIAL_OFFSET='0' \
 -e SIMULATOR_NUMBER='100' \
 -e SIMULATOR_INTERVAL='60' \
 btrdb/simulator:latest
