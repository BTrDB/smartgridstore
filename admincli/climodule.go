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
