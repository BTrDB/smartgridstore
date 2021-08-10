// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package admincli

import (
	"context"
	"io"
)

type key int

const MaxHintLength = 50
const ConsoleWidth key = 0

type CLIModule interface {
	//The children under this module. Invalid if runnable = true
	Children() []CLIModule

	//The short name of this module. e.g "adduser"
	Name() string

	//The help hint for this module. This will be truncated to
	//MaxHintLength characters, so keep it short
	Hint() string

	//The full help text for the module. No length restrictions.
	//Try not to introduce artificial line breaks, the text will
	//be wrapped automatically (one line per paragraph is good)
	Usage() string

	//Is this a command? false if it is a category
	Runnable() bool

	//Run this module. Return when the command is done.
	//Do not write to output after returning
	//The context will contain the final window width as well.
	//ctx.Value(adminCli.ConsoleWidth) will be an integer. -1 if unknown
	//and >1 if the width is known
	Run(ctx context.Context, output io.Writer, args ...string) (argsOk bool)
}

type GenericCLIModule struct {
	MChildren []CLIModule
	MName     string
	MHint     string
	MUsage    string
	MRunnable bool
	MRun      func(context.Context, io.Writer, ...string) bool
}

func (g *GenericCLIModule) Children() []CLIModule {
	return g.MChildren
}

func (g *GenericCLIModule) Name() string {
	return g.MName
}

func (g *GenericCLIModule) Hint() string {
	return g.MHint
}

func (g *GenericCLIModule) Usage() string {
	return g.MUsage
}

func (g *GenericCLIModule) Runnable() bool {
	return g.MRunnable
}

func (g *GenericCLIModule) Run(ctx context.Context, output io.Writer, args ...string) (argsOk bool) {
	return g.MRun(ctx, output, args...)
}
