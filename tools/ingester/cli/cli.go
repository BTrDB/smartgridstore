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
	"strings"

	"github.com/SoftwareDefinedBuildings/mr-plotter/accounts"
	"github.com/immesys/smartgridstore/admincli"

	etcd "github.com/coreos/etcd/clientv3"
)

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
		&ManifestCommand{
			name:      "adduser",
			usageargs: "username password [tag1] [tag2] ...",
			hint:      "creates a new user account",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) >= 2; !argsOK {
					return
				}
				tagSet := sliceToSet(tokens[2:])
				tagSet[accounts.PUBLIC_TAG] = struct{}{}
				acc := &accounts.MrPlotterAccount{Username: tokens[0], Tags: tagSet}
				acc.SetPassword([]byte(tokens[1]))
				success, err := accounts.UpsertAccountAtomically(ctx, etcdClient, acc)
				if !success {
					writeStringln(output, alreadyExists)
					return
				}
				writeError(output, err)
				return
			},
		},
		&ManifestCommand{
			name:      "setpassword",
			usageargs: "username password",
			hint:      "sets a user's password",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) == 2; !argsOK {
					return
				}
				acc, err := accounts.RetrieveAccount(ctx, etcdClient, tokens[0])
				if waserr, _ := writeError(output, err); waserr {
					return
				}
				acc.SetPassword([]byte(tokens[1]))
				success, err := accounts.UpsertAccountAtomically(ctx, etcdClient, acc)
				if !success {
					writeStringln(output, txFail)
					return
				}
				writeError(output, err)
				return
			},
		},
		&ManifestCommand{
			name:      "rmuser",
			usageargs: "username1 [username2] [username3 ...]",
			hint:      "deletes user accounts",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) >= 1; !argsOK {
					return
				}
				for _, username := range tokens {
					err := accounts.DeleteAccount(ctx, etcdClient, username)
					if waserr, _ := writeError(output, err); waserr {
						return
					}
				}
				return
			},
		},
		&ManifestCommand{
			name:      "rmusers",
			usageargs: "usernameprefix",
			hint:      "deletes all user accounts with a certain prefix",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) == 1; !argsOK {
					return
				}
				n, err := accounts.DeleteMultipleAccounts(ctx, etcdClient, tokens[0])
				if n == 1 {
					writeStringln(output, "Deleted 1 account")
				} else {
					writeStringf(output, "Deleted %v accounts\n", n)
				}
				writeError(output, err)
				return
			},
		},
		&ManifestCommand{
			name:      "grant",
			usageargs: "username tag1 [tag2] [tag3] ...",
			hint:      "grants permission to view streams with given tags",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) >= 2; !argsOK {
					return
				}
				acc, err := accounts.RetrieveAccount(ctx, etcdClient, tokens[0])
				if waserr, _ := writeError(output, err); waserr {
					return
				}
				for _, tag := range tokens[1:] {
					if _, ok := acc.Tags[tag]; !ok {
						acc.Tags[tag] = struct{}{}
					}
				}
				success, err := accounts.UpsertAccountAtomically(ctx, etcdClient, acc)
				if !success {
					writeStringln(output, txFail)
					return
				}
				writeError(output, err)
				return
			},
		},
		&ManifestCommand{
			name:      "revoke",
			usageargs: "username tag1 [tag2] [tag3] ...",
			hint:      "revokes tags from a user's permission list",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) >= 2; !argsOK {
					return
				}
				acc, err := accounts.RetrieveAccount(ctx, etcdClient, tokens[0])
				if waserr, _ := writeError(output, err); waserr {
					return
				}
				for _, tag := range tokens[1:] {
					if tag == accounts.PUBLIC_TAG {
						writeStringf(output, "All user accounts must be assigned the \"%s\" tag\n", accounts.PUBLIC_TAG)
						return
					}
					if _, ok := acc.Tags[tag]; ok {
						delete(acc.Tags, tag)
					}
				}
				success, err := accounts.UpsertAccountAtomically(ctx, etcdClient, acc)
				if !success {
					writeStringln(output, txFail)
					return
				}
				writeError(output, err)
				return
			},
		},
		&ManifestCommand{
			name:      "showuser",
			usageargs: "username1 [username2] [username3] ...",
			hint:      "shows the tags granted to a user or users",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) >= 1; !argsOK {
					return
				}
				for _, username := range tokens {
					acc, err := accounts.RetrieveAccount(ctx, etcdClient, username)
					if waserr, _ := writeError(output, err); waserr {
						return
					}
					tagSlice := setToSlice(acc.Tags)
					writeStringf(output, "%s: %s\n", username, strings.Join(tagSlice, " "))
				}
				return
			},
		},
		&ManifestCommand{
			name:      "lsusers",
			usageargs: "[prefix]",
			hint:      "shows the tags granted to all user accounts with a given prefix",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) == 0 || len(tokens) == 1; !argsOK {
					return
				}

				prefix := ""
				if len(tokens) == 1 {
					prefix = tokens[0]
				}

				accs, err := accounts.RetrieveMultipleAccounts(ctx, etcdClient, prefix)
				if waserr, _ := writeError(output, err); waserr {
					return
				}

				for _, acc := range accs {
					if acc.Tags == nil {
						writeStringf(output, "%s [CORRUPT ENTRY]\n", acc.Username)
					} else {
						tagSlice := setToSlice(acc.Tags)
						writeStringf(output, "%s: %s\n", acc.Username, strings.Join(tagSlice, " "))
					}
				}
				return
			},
		},
		&ManifestCommand{
			name:      "deftag",
			usageargs: "tag pathprefix1 [pathprefix2] ...",
			hint:      "defines a new tag",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) >= 2; !argsOK {
					return
				}
				if tokens[0] == accounts.ALL_TAG {
					writeStringln(output, alreadyExists)
					return
				}
				pfxSet := sliceToSet(tokens[1:])
				tagdef := &accounts.MrPlotterTagDef{Tag: tokens[0], PathPrefix: pfxSet}
				success, err := accounts.UpsertTagDefAtomically(ctx, etcdClient, tagdef)
				if !success {
					writeStringln(output, alreadyExists)
					return
				}
				writeError(output, err)
				return
			},
		},
		&ManifestCommand{
			name:      "undeftag",
			usageargs: "tag1 [tag2] [tag3] ...",
			hint:      "deletes tag definitions",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) >= 1; !argsOK {
					return
				}
				for _, tagname := range tokens {
					if tagname == accounts.ALL_TAG {
						writeStringf(output, "Tag \"%s\" cannot be deleted\n", accounts.ALL_TAG)
						return
					}
					err := accounts.DeleteTagDef(ctx, etcdClient, tagname)
					if waserr, _ := writeError(output, err); waserr {
						return
					}
				}
				return
			},
		},
		&ManifestCommand{
			name:      "undeftags",
			usageargs: "prefix",
			hint:      "deletes tag definitions beginning with a certain prefix",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) == 1; !argsOK {
					return
				}
				n, err := accounts.DeleteMultipleTagDefs(ctx, etcdClient, tokens[0])
				if n == 1 {
					writeStringln(output, "Deleted 1 tag definition")
				} else {
					writeStringf(output, "Deleted %v tag definitions\n", n)
				}
				writeError(output, err)
				return
			},
		},
		&ManifestCommand{
			name:      "addprefix",
			usageargs: "tag prefix1 [prefix2] [prefix3] ...",
			hint:      "adds a path prefix to a tag definition",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) >= 2; !argsOK {
					return
				}
				if tokens[0] == accounts.ALL_TAG {
					writeStringf(output, "Cannot modify definition of \"%s\" tag\n", accounts.ALL_TAG)
					return
				}
				tagdef, err := accounts.RetrieveTagDef(ctx, etcdClient, tokens[0])
				if waserr, _ := writeError(output, err); waserr {
					return
				}
				for _, pfx := range tokens[1:] {
					if _, ok := tagdef.PathPrefix[pfx]; !ok {
						tagdef.PathPrefix[pfx] = struct{}{}
					}
				}
				success, err := accounts.UpsertTagDefAtomically(ctx, etcdClient, tagdef)
				if !success {
					writeStringln(output, txFail)
					return
				}
				writeError(output, err)
				return
			},
		},
		&ManifestCommand{
			name:      "rmprefix",
			usageargs: "tag prefix1 [prefix2] [prefix3] ...",
			hint:      "removes a path prefix from a tag definition",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) >= 2; !argsOK {
					return
				}
				if tokens[0] == accounts.ALL_TAG {
					writeStringf(output, "Cannot modify definition of \"%s\" tag\n", accounts.ALL_TAG)
					return
				}
				tagdef, err := accounts.RetrieveTagDef(ctx, etcdClient, tokens[0])
				if waserr, _ := writeError(output, err); waserr {
					return
				}
				for _, pfx := range tokens[1:] {
					if _, ok := tagdef.PathPrefix[pfx]; ok {
						if len(tagdef.PathPrefix) == 1 {
							writeStringln(output, "Each tag must be assigned at least one prefix (use undeftag or undeftags to fully remove a tag)")
							return
						}
						delete(tagdef.PathPrefix, pfx)
					}
				}
				success, err := accounts.UpsertTagDefAtomically(ctx, etcdClient, tagdef)
				if !success {
					writeStringln(output, txFail)
					return
				}
				writeError(output, err)
				return
			},
		},
		&ManifestCommand{
			name:      "showtagdef",
			usageargs: "tag1 [tag2] [tag3] ...",
			hint:      "lists the prefixes assigned to a tag",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) >= 1; !argsOK {
					return
				}
				for _, tagname := range tokens {
					if tagname == accounts.ALL_TAG {
						writeStringf(output, "%s: %s\n", accounts.ALL_TAG, AllTagSymbol)
						continue
					}
					tagdef, err := accounts.RetrieveTagDef(ctx, etcdClient, tagname)
					if waserr, _ := writeError(output, err); waserr {
						return
					}
					pfxSlice := setToSlice(tagdef.PathPrefix)
					writeStringf(output, "%s: %s\n", tagname, strings.Join(pfxSlice, " "))
				}
				return
			},
		},
		&ManifestCommand{
			name:      "lstagdefs",
			usageargs: "[tagprefix]",
			hint:      "lists the prefixes assigned to all tags beginning with a given prefix",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) == 0 || len(tokens) == 1; !argsOK {
					return
				}

				prefix := ""
				if len(tokens) == 1 {
					prefix = tokens[0]
				}

				tagdefs, err := accounts.RetrieveMultipleTagDefs(ctx, etcdClient, prefix)
				if waserr, _ := writeError(output, err); waserr {
					return
				}

				if strings.HasPrefix(accounts.ALL_TAG, prefix) {
					writeStringf(output, "%s: %s\n", accounts.ALL_TAG, AllTagSymbol)
				}
				for _, tagdef := range tagdefs {
					if tagdef.Tag == accounts.ALL_TAG {
						continue
					}
					if tagdef.PathPrefix == nil {
						writeStringf(output, "%s [CORRUPT ENTRY]\n", tagdef.Tag)
					} else {
						pfxSlice := setToSlice(tagdef.PathPrefix)
						writeStringf(output, "%s: %s\n", tagdef.Tag, strings.Join(pfxSlice, " "))
					}
				}
				return
			},
		},
		&ManifestCommand{
			name:      "lsconf",
			usageargs: "[prefix]",
			hint:      "lists the path prefixes currently visible to each user",
			exec: func(ctx context.Context, output io.Writer, tokens ...string) (argsOK bool) {
				if argsOK = len(tokens) == 0 || len(tokens) == 1; !argsOK {
					return
				}

				prefix := ""
				if len(tokens) == 1 {
					prefix = tokens[0]
				}

				accs, err := accounts.RetrieveMultipleAccounts(ctx, etcdClient, prefix)
				if waserr, _ := writeError(output, err); waserr {
					return
				}

				tagcache := make(map[string]map[string]struct{})

				for _, acc := range accs {
					if acc.Tags == nil {
						writeStringf(output, "%s [CORRUPT ENTRY]\n", acc.Username)
					} else {
						prefixes := make(map[string]struct{})
						for tag := range acc.Tags {
							var tagPfxSet map[string]struct{}
							var ok bool

							/* The ALL tag overrides everything. */
							if tag == accounts.ALL_TAG {
								prefixes = make(map[string]struct{})
								prefixes[AllTagSymbol] = struct{}{}
								break
							}

							if tagPfxSet, ok = tagcache[tag]; !ok {
								tagdef, err := accounts.RetrieveTagDef(ctx, etcdClient, tag)
								if err != nil {
									writeStringf(output, "Could not retrieve tag information for '%s': %v\n", tag, err)
									return
								}
								tagPfxSet = tagdef.PathPrefix
							}
							for pfx := range tagPfxSet {
								prefixes[pfx] = struct{}{}
							}
						}
						writeStringf(output, "%s: %s\n", acc.Username, strings.Join(setToSlice(prefixes), " "))
					}
				}
				return
			},
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
	return `This tool allows you to configure Mr. Plotter.
		Account configuration works as follows. A user account consists of a username, password, and set of permissions. The set of permissions describes the streams viewable by that user.
		Permissions can be described at the granularity of collections in BTrDB. In other words, for each collection, a user has either permission to view all of the streams in the collection, or does not have permission to view any of them.
		A tag represents a set of permissions that can be granted to a user. Each tag is defined by a list of collection prefixes, and represents permissions to view streams belonging to collections beginning with one of those prefixes.
		The permissions granted to a user are defined by a list of tags. A user can view a stream if and only if one of the tags assigned to that user grants permission to view that stream.
		There are two special tags: "public" and "all". The "public" tag describes the streams viewable to users who are not logged in. The "all" tag grants access to all streams.
		`
}

// Runnable returns false.
func (mpcli *MrPlotterCLIModule) Runnable() bool {
	return false
}

// Run does nothing and returns false.
func (mpcli *MrPlotterCLIModule) Run(ctx context.Context, output io.Writer, args ...string) (argsOk bool) {
	return false
}
