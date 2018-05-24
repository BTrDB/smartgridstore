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
	"encoding/base64"
	"fmt"
	"io"

	"github.com/BTrDB/mr-plotter/keys"
	"github.com/BTrDB/smartgridstore/admincli"

	etcd "github.com/coreos/etcd/clientv3"
)

// MrPlotterCommand encapsulates a CLI command.
type MrPlotterCommand struct {
	name      string
	usageargs string
	hint      string
	exec      func(ctx context.Context, output io.Writer, tokens ...string) bool
}

// Children return nil.
func (mpc *MrPlotterCommand) Children() []admincli.CLIModule {
	return nil
}

// Name returns the name of the command.
func (mpc *MrPlotterCommand) Name() string {
	return mpc.name
}

// Hint returns a short help text string for the command.
func (mpc *MrPlotterCommand) Hint() string {
	return mpc.hint
}

// Usage returns a longer help text string for the command.
func (mpc *MrPlotterCommand) Usage() string {
	return fmt.Sprintf(" %s\nThis command %s.\n", mpc.usageargs, mpc.hint)
}

// Runnable returns true, as MrPlotterCommand encapsulates a CLI command.
func (mpc *MrPlotterCommand) Runnable() bool {
	return true
}

// Run executes the CLI command encapsulated by this MrPlotterCommand.
func (mpc *MrPlotterCommand) Run(ctx context.Context, output io.Writer, args ...string) (argsOk bool) {
	return mpc.exec(ctx, output, args...)
}

const txFail = "Transacation for atomic update failed; try again"
const alreadyExists = "Already exists"
const accountNotExists = "Account does not exist"
const tagNotExists = "Tag is not defined"

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

// MrPlotterCLIModule encapsulates the CLI module for configuring Mr. Plotter.
type MrPlotterCLIModule struct {
	ecl *etcd.Client
}

// NewMrPlotterCLIModule returns a new instance of MrPlotterCLIModule.
func NewMrPlotterCLIModule(ecl *etcd.Client) *MrPlotterCLIModule {
	return &MrPlotterCLIModule{ecl}
}

// Children returns the CLI functions for the Mr. Plotter CLI module.
func (mpcli *MrPlotterCLIModule) Children() []admincli.CLIModule {
	etcdClient := mpcli.ecl
	return []admincli.CLIModule{
		&admincli.GenericCLIModule{
			MChildren: []admincli.CLIModule{
				&MrPlotterCommand{
					name:      "setcertsrc",
					usageargs: "source",
					hint:      "sets the method by which the certificate is obtained",
					exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
						if argsOK = len(tokens) == 1; !argsOK {
							return
						}
						source := tokens[0]
						if source != "autocert" && source != "hardcoded" && source != "config" {
							writeStringf(output, "Argument to setcertsrc must be \"autocert\", \"hardcoded\", or \"config\"; got \"%v\"\n", source)
							return
						}
						err := keys.SetCertificateSource(ctx, etcdClient, source)
						if err != nil {
							writeStringf(output, "Could not set certificate source: %v\n", err)
						}
						return
					},
				},
				&MrPlotterCommand{
					name:      "getcertsrc",
					usageargs: "",
					hint:      "gets the method by which the certificate is obtained",
					exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
						if argsOK = len(tokens) == 0; !argsOK {
							return
						}
						source, err := keys.GetCertificateSource(ctx, etcdClient)
						if err != nil {
							writeStringf(output, "Could not get certificate source: %v\n", err)
						}
						writeStringln(output, source)
						return
					},
				},
				&admincli.GenericCLIModule{
					MChildren: []admincli.CLIModule{
						&MrPlotterCommand{
							name:      "sethost",
							usageargs: "hostname",
							hint:      "sets the hostname for autocert",
							exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
								if argsOK = len(tokens) == 1; !argsOK {
									return
								}
								err := keys.SetAutocertHostname(ctx, etcdClient, tokens[0])
								if err != nil {
									writeStringf(output, "Could not set autocert host: %v\n", err)
								}
								return
							},
						},
						&MrPlotterCommand{
							name:      "setemail",
							usageargs: "email",
							hint:      "sets the email address for autocert",
							exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
								if argsOK = len(tokens) == 1; !argsOK {
									return
								}
								err := keys.SetAutocertEmail(ctx, etcdClient, tokens[0])
								if err != nil {
									writeStringf(output, "Could not set autocert email: %v\n", err)
								}
								return
							},
						},
						&MrPlotterCommand{
							name:      "show",
							usageargs: "",
							hint:      "shows autocert information",
							exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
								if argsOK = len(tokens) == 0; !argsOK {
									return
								}
								hostname, err := keys.GetAutocertHostname(ctx, etcdClient)
								if err != nil {
									writeStringf(output, "Could not get autocert hostname: %v\n", err)
								}
								email, err := keys.GetAutocertEmail(ctx, etcdClient)
								if err != nil {
									writeStringf(output, "Could not get autocert email: %v\n", err)
								}
								writeStringf(output, "Hostname: %s\nEmail: %s\n", hostname, email)
								return
							},
						},
					},
					MName:     "autocert",
					MHint:     "configure autocert",
					MUsage:    "configure autocert",
					MRunnable: false,
					MRun:      nil,
				},
				&MrPlotterCommand{
					name:      "sethardcoded",
					usageargs: "cert key",
					hint:      "sets the certificate to use when the source is set to \"hardcoded\"",
					exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
						if argsOK = len(tokens) == 2; !argsOK {
							return
						}
						cert, err := base64.StdEncoding.DecodeString(tokens[0])
						if err != nil {
							writeStringf(output, "cert is not properly base64 encoded: %v\n", err)
							return
						}
						key, err := base64.StdEncoding.DecodeString(tokens[1])
						if err != nil {
							writeStringf(output, "key is not properly base64 encoded: %v\n", err)
							return
						}
						htls := &keys.HardcodedTLSCertificate{Cert: cert, Key: key}
						err = keys.UpsertHardcodedTLSCertificate(ctx, etcdClient, htls)
						if err != nil {
							writeStringf(output, "Could not set hardcoded certificate: %v\n", err)
						}
						return
					},
				},
				&MrPlotterCommand{
					name:      "gethardcoded",
					usageargs: "",
					hint:      "gets the certificate when the source is set to \"hardcoded\"",
					exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
						if argsOK = len(tokens) == 0; !argsOK {
							return
						}
						htls, err := keys.RetrieveHardcodedTLSCertificate(ctx, etcdClient)
						if err != nil {
							writeStringf(output, "Could not get hardcoded certificate: %v\n", err)
							return
						}
						var cert string
						var key string
						if htls != nil {
							cert = string(htls.Cert)
							key = string(htls.Key)
						}
						writeStringf(output, "%s%s", cert, key)
						return
					},
				},
				&MrPlotterCommand{
					name:      "setsessionkeys",
					usageargs: "encryptkey mackey",
					hint:      "sets the session keys",
					exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
						if argsOK = len(tokens) == 2; !argsOK {
							return
						}
						encrypt, err := base64.StdEncoding.DecodeString(tokens[0])
						if err != nil {
							writeStringf(output, "encryptkey is not properly base64 encoded: %v\n", err)
							return
						}
						mac, err := base64.StdEncoding.DecodeString(tokens[1])
						if err != nil {
							writeStringf(output, "mackey is not properly base64 encoded: %v\n", err)
							return
						}
						sk := &keys.SessionKeys{EncryptKey: encrypt, MACKey: mac}
						err = keys.UpsertSessionKeys(ctx, etcdClient, sk)
						if err != nil {
							writeStringf(output, "Could not set session keys: %v\n", err)
						}
						return
					},
				},
				&MrPlotterCommand{
					name:      "getsessionkeys",
					usageargs: "",
					hint:      "gets the session keys",
					exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
						if argsOK = len(tokens) == 0; !argsOK {
							return
						}
						sk, err := keys.RetrieveSessionKeys(ctx, etcdClient)
						if err != nil {
							writeStringf(output, "Could not get session keys: %v\n", err)
							return
						}
						var encrypt string
						var mac string
						if sk != nil {
							encrypt = base64.StdEncoding.EncodeToString(sk.EncryptKey)
							mac = base64.StdEncoding.EncodeToString(sk.MACKey)
						}
						writeStringf(output, "Encrypt Key: %s\nMAC Key: %s\n", encrypt, mac)
						return
					},
				},
			},
			MName:     "keys",
			MHint:     "Manage session and HTTPS keys",
			MUsage:    "Manage session and HTTPS keys",
			MRunnable: false,
			MRun:      nil,
		},
	}
}

// Name returns "mrplotter"
func (mpcli *MrPlotterCLIModule) Name() string {
	return "mrplotter"
}

// Hint prints the hint string for the Mr. Plotter CLI Module.
func (mpcli *MrPlotterCLIModule) Hint() string {
	return "configure Mr. Plotter"
}

// Usage returns the usage string for the Mr. Plotter CLI Module.
func (mpcli *MrPlotterCLIModule) Usage() string {
	return `This tool allows you to configure Mr. Plotter.`
}

// Runnable returns false.
func (mpcli *MrPlotterCLIModule) Runnable() bool {
	return false
}

// Run does nothing and returns false.
func (mpcli *MrPlotterCLIModule) Run(ctx context.Context, output io.Writer, args ...string) (argsOk bool) {
	return false
}
