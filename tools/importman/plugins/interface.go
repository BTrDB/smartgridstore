// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package plugins

type DataSource interface {

	//Next returns the next set of streams. Streams with the same collection
	//and tags will be merged with previously returned streams so it is acceptable
	//to return a stream more than once as long as the data within those streams
	//does not overlap
	Next() []Stream
	Total() (total int64, totalKnown bool)
}

type Stream interface {

	//The CollectionSuffix is what will be appended onto the user specified
	//destination collection. It can be an empty string as long as the Tags
	//are unique for all streams, otherwise the combination of CollectionSuffix
	//and Tags must be unique
	CollectionSuffix() string

	//The Tags form part of the identity of the stream. Specifically if there
	//is a `name` tag, it is used in the plotter as the final element of the
	//tree.
	Tags() map[string]string

	//Annotations contain additional metadata that is associated with the stream
	//but is changeable or otherwise not suitable for identifying the stream
	Annotations() map[string]string

	//Next returns a chunk of data for insertion. If the data is empty it is
	//assumed that there is no more data to insert
	Next() (data []Point)

	//Total returns the total number of datapoints, used for progress estimation.
	//If no total is available, return 0, false
	Total() (total int64, totalKnown bool)
}

type Point struct {
	//Nanoseconds since the unix epoch
	Time  int64
	Value float64
}
