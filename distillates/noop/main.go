// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

import (
	"fmt"
	"os"

	"gopkg.in/distil.v4"
)

func main() {

	//Parameters come from environment variables
	inputpath := os.Getenv("INPUT_PATH")
	outputpath := os.Getenv("OUTPUT_PATH")
	id := os.Getenv("DISTILLATE_ID")
	if inputpath == "" || outputpath == "" || id == "" {
		fmt.Printf("All of $DISTILLATE_ID, $INPUT_PATH and $OUTPUT_PATH are required")
		os.Exit(1)
	}

	// Get a handle to BTrDB. go-distil is implemented as a library
	// so there is no other distillate service to connect to
	ds := distil.NewDISTIL()

	// Construct an instance of your distillate. If you had parameters for
	// the distillate you would maybe have a custom constructor. You could
	// also load the parameters from a config file, or some heuristic
	// algorithm, which we will show in the next few examples
	instance := &NopDistiller{}

	// Now we add this distillate to the DISTIL engine. If you add multiple
	// distillates, they will all get computed in parallel.
	ds.RegisterDistillate(&distil.Registration{
		// The class that implements your algorithm
		Instance: instance,
		// A unique name FOR THIS INSTANCE of the distillate. If you
		// are autogenerating distillates, take care to never produce
		// the same name here. We would normally use a UUID but opted
		// for this so as to be more human friendly. If the program
		// is restarted, this is how it knows where to pick up from.
		UniqueName: id,
		// These are inputs to the distillate that will be loaded
		// and presented to Process()
		InputPaths: []string{inputpath},
		// These are the output paths for the distillate. They must
		// also be strictly unique.
		OutputPaths: []string{outputpath},
		// These are the units for the output paths
		OutputUnits: []string{"degrees"},
	})

	//Now we tell the DISTIL library to keep all the registered distillates
	//up to date. The program will not exit.
	ds.StartEngine()
}
