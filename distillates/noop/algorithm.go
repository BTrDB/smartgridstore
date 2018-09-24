package main

import "gopkg.in/distil.v4"

//This is our distillate algorithm
type NopDistiller struct {
	// This line is required. It says this struct inherits some useful
	// default methods.
	distil.DistillateTools

	// We keep it simple for example 1. You could put other variables here
	// if you wished
}

// This is our main algorithm. It will automatically be called with chunks
// of data that require processing by the engine.
func (d *NopDistiller) Process(in *distil.InputSet, out *distil.OutputSet) {
	// The InputSet contains "samples". These are unadulterated samples by
	// default (possibly containing holes or duplicate values), but later
	// examples will look at some of the preconditioning tools available to you.
	// our simple NOOP distillate simply maintains an exact copy of the
	// target stream
	for index := 0; index < in.NumSamples(0); index++ {
		out.AddPoint(0, in.Get(0, index))
	}
}
