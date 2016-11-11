#!/bin/bash
ln -s /etc/sync/serial_aliases.ini
receiver -m $MONGO |& pp
