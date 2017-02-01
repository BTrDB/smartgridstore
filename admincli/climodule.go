package admincli

const MaxHintLength = 50

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

	//Run this module. The output channel will write
	//to the user. If argsOk is not true, output will be ignored
	//and the Usage() string will be sent to the user. For more
	//complex argument problems (nonexistent object for example),
	//return argsOk true and print more detailed messages on
	//the output. Always close the output channel when the command
	//is complete. Do not read any input from the user.
	Run(output chan string, args ...string) (argsOk bool)
}
