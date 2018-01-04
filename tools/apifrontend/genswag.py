#!/usr/bin/env python

f = open("swagger.json.go","w")
f.write("package main\n")
swag = open("../../../../../gopkg.in/BTrDB/btrdb.v4/grpcinterface/btrdb.swagger.json").read()
f.write("const SwaggerJSON = `")
f.write(swag)
f.write("`;")
f.close()
