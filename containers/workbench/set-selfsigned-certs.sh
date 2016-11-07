#!/bin/bash

openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout /etc/mrplotter/cert.key -out /etc/mrplotter/cert.crt
sed -i.bak "s/cert_file=.*$/cert_file=\/etc\/mrplotter\/cert.crt/g" /etc/mrplotter/plotter.ini
sed -i.bak "s/key_file=.*$/key_file=\/etc\/mrplotter\/cert.key/g" /etc/mrplotter/plotter.ini
