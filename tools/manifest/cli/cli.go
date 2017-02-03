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

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/immesys/smartgridstore/admincli"
	"github.com/immesys/smartgridstore/tools/manifest"

	etcd "github.com/coreos/etcd/clientv3"
)

const devNotExist = "Device does not exist in the manifest"

// ManifestCommand encapsulates a CLI command.
type ManifestCommand struct {
	name      string
	usageargs string
	hint      string
	exec      func(ctx context.Context, output io.Writer, tokens ...string) bool
}

// Children return nil.
func (mc *ManifestCommand) Children() []admincli.CLIModule {
	return nil
}

// Name returns the name of the command.
func (mc *ManifestCommand) Name() string {
	return mc.name
}

// Hint returns a short help text string for the command.
func (mc *ManifestCommand) Hint() string {
	return mc.hint
}

// Usage returns a longer help text string for the command.
func (mc *ManifestCommand) Usage() string {
	return fmt.Sprintf(" %s\nThis command %s.\n", mc.usageargs, mc.hint)
}

// Runnable returns true, as MrPlotterCommand encapsulates a CLI command.
func (mc *ManifestCommand) Runnable() bool {
	return true
}

// Run executes the CLI command encapsulated by this ManifestCommand.
func (mc *ManifestCommand) Run(ctx context.Context, output io.Writer, args ...string) (argsOk bool) {
	return mc.exec(ctx, output, args...)
}

// AllTagSymbol is the symbol that is used to denote the streams accessible with
// the "all" tag.
const AllTagSymbol = "<ALL STREAMS>"

const txFail = "Transacation for atomic update failed; try again"
const alreadyExists = "Already exists"

func sliceToSet(tagSlice []string) map[string]struct{} {
	tagSet := make(map[string]struct{})
	for _, tag := range tagSlice {
		tagSet[tag] = struct{}{}
	}
	return tagSet
}

func setToSlice(tagSet map[string]struct{}) []string {
	tagSlice := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tagSlice = append(tagSlice, tag)
	}
	return tagSlice
}

func writeStringln(output io.Writer, message string) error {
	_, err := fmt.Fprintln(output, message)
	return err
}

func writeStringf(output io.Writer, format string, a ...interface{}) error {
	_, err := fmt.Fprintf(output, format, a...)
	return err
}

func writeError(output io.Writer, err error) (bool, error) {
	var err2 error
	if err != nil {
		err2 = writeStringf(output, "Operation failed: %s\n", err.Error())
	}
	return err != nil, err2
}

// NewMrPlotterCLIModule returns a new instance of MrPlotterCLIModule.
func NewManifestCLIModule(etcdClient *etcd.Client) *admincli.GenericCLIModule {
	return &admincli.GenericCLIModule{
		MName:     "manifest",
		MHint:     "configure registered phasor measurement units",
		MUsage:    "TODO",
		MRunnable: false,
		MRun: func(ctx context.Context, output io.Writer, arguments ...string) bool {
			return false
		},
		MChildren: []admincli.CLIModule{
			&ManifestCommand{
				name:      "add",
				usageargs: "descriptor [key1=value1] [key2=value2] ...",
				hint:      "creates a new device with the provided descriptor",
				exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
					if argsOK = len(tokens) >= 1; !argsOK {
						return
					}
					metadata := make(map[string]string)
					for _, kv := range tokens[1:] {
						kvslice := strings.Split(kv, "=")
						if len(kvslice) != 2 {
							writeStringln(output, "metadata must be of the form key=value")
							return
						}
						metadata[kvslice[0]] = kvslice[1]
					}
					dev := &manifest.ManifestDevice{Descriptor: tokens[0], Metadata: metadata, Streams: make(map[string]*manifest.ManifestDeviceStream)}
					success, err := manifest.UpsertManifestDeviceAtomically(ctx, etcdClient, dev)
					if !success {
						writeStringln(output, alreadyExists)
						return
					}
					writeError(output, err)
					return
				},
			},
			&ManifestCommand{
				name:      "del",
				usageargs: "descriptor1 [descriptor2] [descriptor3] ...",
				hint:      "deletes devices from the manifest",
				exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
					if argsOK = len(tokens) >= 1; !argsOK {
						return
					}
					for _, descriptor := range tokens {
						err := manifest.DeleteManifestDevice(ctx, etcdClient, descriptor)
						if waserr, _ := writeError(output, err); waserr {
							return
						}
					}
					return
				},
			},
			&ManifestCommand{
				name:      "delprefix",
				usageargs: "descriptorprefix",
				hint:      "deletes all devices with a certain prefix",
				exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
					if argsOK = len(tokens) == 1; !argsOK {
						return
					}
					n, err := manifest.DeleteMultipleManifestDevices(ctx, etcdClient, tokens[0])
					if n == 1 {
						writeStringln(output, "Deleted 1 device")
					} else {
						writeStringf(output, "Deleted %v devices\n", n)
					}
					writeError(output, err)
					return
				},
			},
			&ManifestCommand{
				name:      "setmeta",
				usageargs: "descriptor[/streamname] key1=value1 [key2=value2] [key3=value3] ...",
				hint:      "set metadata",
				exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
					if argsOK = len(tokens) >= 2; !argsOK {
						return
					}
					dparts := strings.Split(tokens[0], "/")

					var desc string
					var streamname string
					if len(dparts) == 1 {
						desc = tokens[0]
						streamname = ""
					} else {
						desc = dparts[0]
						streamname = dparts[1]
					}
					if len(dparts) > 2 {
						writeStringln(output, "First argument must be either of the form \"descriptor\" or \"descriptor/streamname\"")
						return
					}

					dev, err := manifest.RetrieveManifestDevice(ctx, etcdClient, desc)
					if waserr, _ := writeError(output, err); waserr {
						return
					}
					if dev == nil {
						writeStringln(output, devNotExist)
						return
					}

					for _, kv := range tokens[1:] {
						kvslice := strings.Split(kv, "=")
						if len(kvslice) != 2 {
							writeStringln(output, "metadata must be of the form key=value")
							return
						}
						key := kvslice[0]
						val := kvslice[1]
						if streamname == "" {
							if dev.Metadata == nil {
								writeStringln(output, "device entry is corrupt")
								return
							}
							dev.Metadata[key] = val
						} else {
							if dev.Streams == nil {
								writeStringln(output, "device entry is corrupt")
								return
							}
							stream, ok := dev.Streams[streamname]
							if ok {
								if stream.Metadata == nil {
									stream.Metadata = make(map[string]string)
								}
								stream.Metadata[key] = val
							} else {
								stream = &manifest.ManifestDeviceStream{
									CanonicalName: streamname,
									Metadata:      map[string]string{key: val},
								}
								dev.Streams[streamname] = stream
							}
						}
					}

					success, err := manifest.UpsertManifestDeviceAtomically(ctx, etcdClient, dev)
					if !success {
						writeStringln(output, txFail)
						return
					}
					writeError(output, err)
					return
				},
			},
			&ManifestCommand{
				name:      "delmeta",
				usageargs: "descriptor[/streamname] key1 [key2] [key3] ...",
				hint:      "deletes metadata key-value pairs",
				exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
					if argsOK = len(tokens) >= 2; !argsOK {
						return
					}
					dparts := strings.Split(tokens[0], "/")

					var desc string
					var streamname string
					if len(dparts) == 1 {
						desc = tokens[0]
						streamname = ""
					} else {
						desc = dparts[0]
						streamname = dparts[1]
					}
					if len(dparts) > 2 {
						writeStringln(output, "First argument must be either of the form \"descriptor\" or \"descriptor/streamname\"")
						return
					}

					dev, err := manifest.RetrieveManifestDevice(ctx, etcdClient, desc)
					if waserr, _ := writeError(output, err); waserr {
						return
					}
					if dev == nil {
						writeStringln(output, devNotExist)
						return
					}

					for _, key := range tokens[1:] {
						if streamname == "" {
							delete(dev.Metadata, key)
						} else {
							stream, ok := dev.Streams[streamname]
							if ok {
								delete(stream.Metadata, key)
								if len(stream.Metadata) == 0 {
									delete(dev.Streams, streamname)
								}
							}
						}
					}

					success, err := manifest.UpsertManifestDeviceAtomically(ctx, etcdClient, dev)
					if !success {
						writeStringln(output, txFail)
						return
					}
					writeError(output, err)
					return
				},
			},
			&ManifestCommand{
				name:      "lsdevs",
				usageargs: "[prefix]",
				hint:      "lists metadata for all devices with a given prefix",
				exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
					if argsOK = len(tokens) == 0 || len(tokens) == 1; !argsOK {
						return
					}

					prefix := ""
					if len(tokens) == 1 {
						prefix = tokens[0]
					}

					devs, err := manifest.RetrieveMultipleManifestDevices(ctx, etcdClient, prefix)
					if waserr, _ := writeError(output, err); waserr {
						return
					}

					for _, dev := range devs {
						if dev.Metadata == nil || dev.Streams == nil {
							writeStringf(output, "%s: [CORRUPT ENTRY]\n", dev.Descriptor)
							continue
						}
						marshalled, err := yaml.Marshal(dev)
						if err != nil {
							writeStringln(output, "[CORRUPT ENTRY]")
							continue
						}
						writeStringf(output, "%s\n%s\n%s\n", dev.Descriptor, strings.Repeat("-", len(dev.Descriptor)), string(marshalled))
						fmt.Fprintln(os.Stdout, marshalled)
					}
					return
				},
			},
		},
	}
}
