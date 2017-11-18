/*
 * Copyright (c) 2017, Sam Kumar <samkumar@berkeley.edu>
 * Copyright (c) 2017, University of California, Berkeley
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are met:
 *     * Redistributions of source code must retain the above copyright
 *       notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above copyright
 *       notice, this list of conditions and the following disclaimer in the
 *       documentation and/or other materials provided with the distribution.
 *     * Neither the name of the University of California, Berkeley nor the
 *       names of its contributors may be used to endorse or promote products
 *       derived from this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 * DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNERS OR CONTRIBUTORS BE LIABLE FOR
 * ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
 * LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
 * ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
 * SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/BTrDB/mr-plotter/accounts"
	"github.com/BTrDB/smartgridstore/admincli"
	"github.com/BTrDB/smartgridstore/tools/mr-plotter-conf/cli"

	etcd "github.com/coreos/etcd/clientv3"
)

var mpcli admincli.CLIModule
var ops = make(map[string]admincli.CLIModule)

func main() {
	etcdEndpoint := os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "localhost:2379"
	}
	etcdKeyPrefix := os.Getenv("ETCD_KEY_PREFIX")
	if len(etcdKeyPrefix) != 0 {
		accounts.SetEtcdKeyPrefix(etcdKeyPrefix)
		fmt.Printf("Using Mr. Plotter configuration '%s'\n", etcdKeyPrefix)
	}
	etcdClient, err := etcd.New(etcd.Config{Endpoints: []string{etcdEndpoint}})
	if err != nil {
		fmt.Printf("Could not connect to etcd: %v\n", err)
		os.Exit(1)
	}

	mpcli = cli.NewMrPlotterCLIModule(etcdClient)
	cmds := mpcli.Children()
	for _, cmd := range cmds {
		ops[cmd.Name()] = cmd
	}

	/* Start the REPL. */
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Mr. Plotter> ")
		if !scanner.Scan() {
			break
		}
		result := scanner.Text()
		accountsExec(etcdClient, result)
	}

	fmt.Println()
	if err := scanner.Err(); err != nil {
		fmt.Printf("Exiting: %v\n", err)
	}
}

func help() {
	commands := make([]string, 0, len(ops))
	for _, cmd := range mpcli.Children() {
		commands = append(commands, cmd.Name())
	}
	fmt.Println("Type one of the following commands and press <Enter> or <Return> to execute it:")
	fmt.Println(strings.Join(commands, " "))
}

func accountsExec(etcdClient *etcd.Client, cmd string) {
	tokens := strings.Fields(cmd)
	if len(tokens) == 0 {
		return
	}

	opcode := tokens[0]

	if opcode == "help" {
		help()
		return
	}

	if op, ok := ops[opcode]; ok {
		argsOK := op.Run(context.Background(), os.Stdout, tokens[1:]...)
		if !argsOK {
			fmt.Printf("Usage: %s%s", op.Name(), op.Usage())
		}
	} else {
		fmt.Printf("'%s' is not a valid command\n", opcode)
		help()
	}
}
