// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package acl

import (
	"context"
	"fmt"
	"io"

	"github.com/BTrDB/smartgridstore/admincli"
	etcd "github.com/coreos/etcd/clientv3"
)

type usersModule struct {
	e            *ACLEngine
	c            *etcd.Client
	loggedInUser string
}
type singleUserModule struct {
	e            *ACLEngine
	c            *etcd.Client
	loggedInUser string
	username     string
}

func NewACLModule(c *etcd.Client, loggedInUser string) admincli.CLIModule {
	aclEngine := NewACLEngine("btrdb", c)
	add := func(ctx context.Context, w io.Writer, a ...string) bool {
		return adduser(loggedInUser, aclEngine, ctx, w, a...)
	}
	del := func(ctx context.Context, w io.Writer, a ...string) bool {
		return deluser(loggedInUser, aclEngine, ctx, w, a...)
	}
	users := &usersModule{loggedInUser: loggedInUser, e: aclEngine, c: c}
	return &admincli.GenericCLIModule{
		MName:  "acl",
		MHint:  "manage users and permissions",
		MUsage: "",
		MChildren: []admincli.CLIModule{
			&admincli.GenericCLIModule{
				MName:     "add",
				MHint:     "add a new user",
				MUsage:    " username password\nAdds a new user to the system",
				MRun:      add,
				MRunnable: true,
			},
			&admincli.GenericCLIModule{
				MName:     "del",
				MHint:     "delete a user",
				MUsage:    " username\nDelete a user from the system",
				MRun:      del,
				MRunnable: true,
			},
			&admincli.GenericCLIModule{
				MName:     "addgroup",
				MHint:     "add a new group",
				MUsage:    " groupname",
				MRunnable: true,
				MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
					fmt.Printf("args len is %d\n", len(args))
					if len(args) != 1 {
						return false
					}
					err := aclEngine.AddGroup(args[0])
					if err != nil {
						fmt.Fprintf(w, "failed: %v\n", err)
					}
					return true
				},
			},
			&admincli.GenericCLIModule{
				MName:     "delgroup",
				MHint:     "remove a group",
				MUsage:    " groupname",
				MRunnable: true,
				MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
					if len(args) != 1 {
						return false
					}
					err := aclEngine.DeleteGroup(args[0])
					if err != nil {
						fmt.Fprintf(w, "failed: %v\n", err)
					}
					return true
				},
			},
			&admincli.GenericCLIModule{
				MName:     "addprefixtogroup",
				MHint:     "add a permitted collection prefix to a group",
				MUsage:    " groupname prefix",
				MRunnable: true,
				MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
					if len(args) != 2 {
						return false
					}
					err := aclEngine.AddPrefixToGroup(args[0], args[1])
					if err != nil {
						fmt.Fprintf(w, "failed: %v\n", err)
					}
					return true
				},
			},
			&admincli.GenericCLIModule{
				MName:     "delprefixfromgroup",
				MHint:     "remove a permitted collection prefix from a group",
				MUsage:    " groupname prefix",
				MRunnable: true,
				MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
					if len(args) != 2 {
						return false
					}
					err := aclEngine.RemovePrefixFromGroup(args[0], args[1])
					if err != nil {
						fmt.Fprintf(w, "failed: %v\n", err)
					}
					return true
				},
			},
			&admincli.GenericCLIModule{
				MName:     "addcapabilitytogroup",
				MHint:     "add a capability (plotter,api,insert,read,delete,obliterate) to a group",
				MUsage:    " groupname capability",
				MRunnable: true,
				MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
					if len(args) != 2 {
						return false
					}
					err := aclEngine.AddCapabilityToGroup(args[0], args[1])
					if err != nil {
						fmt.Fprintf(w, "failed: %v\n", err)
					}
					return true
				},
			},
			&admincli.GenericCLIModule{
				MName:     "delcapabilityfromgroup",
				MHint:     "remove a capability from a group",
				MUsage:    " groupname capability",
				MRunnable: true,
				MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
					if len(args) != 2 {
						return false
					}
					err := aclEngine.RemoveCapabilityFromGroup(args[0], args[1])
					if err != nil {
						fmt.Fprintf(w, "failed: %v\n", err)
					}
					return true
				},
			},
			&admincli.GenericCLIModule{
				MName:     "listgroups",
				MHint:     "lists groups",
				MUsage:    " ",
				MRunnable: true,
				MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
					if len(args) != 0 {
						return false
					}
					grps, err := aclEngine.GetGroups()
					if err != nil {
						fmt.Fprintf(w, "failed: %v\n", err)
						return true
					}
					for _, g := range grps {
						fmt.Fprintf(w, "%s:\n", g.Name)
						fmt.Fprintf(w, " Capabilities: ")
						for _, c := range g.Capabilities {
							fmt.Fprintf(w, "%s ", c)
						}
						fmt.Fprintf(w, "\n Prefixes:\n")
						for _, p := range g.Prefixes {
							fmt.Fprintf(w, "  %q\n", p)
						}
					}
					return true
				},
			},
			users,
		},
	}
}

func adduser(curUser string, e *ACLEngine, ctx context.Context, w io.Writer, args ...string) bool {
	if len(args) != 2 {
		return false
	}
	err := e.CreateUser(args[0], args[1])
	if err != nil {
		fmt.Fprintf(w, "failed: %v\n", err)
		return true
	}
	fmt.Fprintf(w, "user created\n")
	return true
}
func deluser(curUser string, e *ACLEngine, ctx context.Context, w io.Writer, args ...string) bool {
	if len(args) != 1 {
		return false
	}
	if args[0] == curUser {
		fmt.Fprintf(w, "failed: you cannot delete your own account\n")
		return true
	}
	if args[0] == "admin" {
		fmt.Fprintf(w, "failed: you cannot delete the admin account\n")
		return true
	}
	err := e.DeleteUser(args[0])
	if err != nil {
		fmt.Fprintf(w, "failed: %v\n", err)
		return true
	}
	fmt.Fprintf(w, "user deleted\n")
	return true
}

func (um *usersModule) Children() []admincli.CLIModule {
	users, err := um.e.GetAllUsers()
	if err != nil {
		panic(err)
	}
	rv := []admincli.CLIModule{}
	for _, u := range users {
		rv = append(rv, &singleUserModule{loggedInUser: um.loggedInUser, username: u, e: um.e, c: um.c})
	}
	return rv
}

func (u *usersModule) Name() string {
	return "users"
}

func (u *usersModule) Hint() string {
	return "modify existing users"
}

func (u *usersModule) Usage() string {
	return ""
}

func (u *usersModule) Runnable() bool {
	return false
}

func (u *usersModule) Run(ctx context.Context, output io.Writer, args ...string) (argsOk bool) {
	return false
}

func (su *singleUserModule) Children() []admincli.CLIModule {
	return genUserCommands(su)
}
func (su *singleUserModule) Name() string {
	return su.username
}
func (su *singleUserModule) Hint() string {
	return ""
}
func (su *singleUserModule) Usage() string {
	return ""
}
func (su *singleUserModule) Runnable() bool {
	return false
}
func (su *singleUserModule) Run(ctx context.Context, output io.Writer, args ...string) (argsOk bool) {
	return false
}

/*
add <username> <password>
passwd <username> <password>
allow <username> key key=value key=value
describe <username>
revoke <username> key key key
del <username>
list
*/
func genUserCommands(su *singleUserModule) []admincli.CLIModule {
	return []admincli.CLIModule{
		&admincli.GenericCLIModule{
			MName:     "passwd",
			MHint:     "change user password",
			MUsage:    " newpassword",
			MRunnable: true,
			MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
				fmt.Printf("args len is %d\n", len(args))
				if len(args) != 1 {
					return false
				}
				fmt.Printf("logged in user is %q and username is %q\n", su.loggedInUser, su.username)
				if err := su.e.SetPassword(su.username, args[0]); err != nil {
					fmt.Fprintf(w, "failed: %v\n", err)
					return true
				}
				fmt.Fprintf(w, "password changed\n")
				return true
			},
		},
		&admincli.GenericCLIModule{
			MName:     "addtogroup",
			MHint:     "add a user to a group",
			MUsage:    " group [group] ...",
			MRunnable: true,
			MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
				for _, g := range args {
					err := su.e.AddUserToGroup(su.username, g)
					if err != nil {
						fmt.Fprintf(w, "failed: %v\n", err)
						return true
					}
				}
				return true
			},
		},
		&admincli.GenericCLIModule{
			MName:     "removefromgroup",
			MHint:     "remove a user from a group",
			MUsage:    " group [group] ...",
			MRunnable: true,
			MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
				for _, g := range args {
					err := su.e.RemoveUserFromGroup(su.username, g)
					if err != nil {
						fmt.Fprintf(w, "failed: %v\n", err)
						return true
					}
				}
				return true
			},
		},
		&admincli.GenericCLIModule{
			MName:     "getapikey",
			MHint:     "get the user's api key",
			MUsage:    "",
			MRunnable: true,
			MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
				apikey, err := su.e.GetAPIKey(su.username)
				if err != nil {
					fmt.Fprintf(w, "failed: %v\n", err)
					return true
				}
				fmt.Fprintf(w, "%s\n", apikey)
				return true
			},
		},
		&admincli.GenericCLIModule{
			MName:     "resetapikey",
			MHint:     "reset the user's api key",
			MUsage:    "",
			MRunnable: true,
			MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
				apikey, err := su.e.ResetAPIKey(su.username)
				if err != nil {
					fmt.Fprintf(w, "failed: %v\n", err)
					return true
				}
				fmt.Fprintf(w, "%s\n", apikey)
				return true
			},
		},
		&admincli.GenericCLIModule{
			MName:     "describe",
			MHint:     "print user info",
			MUsage:    "",
			MRunnable: true,
			MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
				u, err := su.e.GetBuiltinUser(su.username)
				if err != nil {
					fmt.Fprintf(w, "failed: %v\n", err)
					return true
				}
				fmt.Fprintf(w, "Groups: \n")
				for _, g := range u.FullGroups {
					fmt.Fprintf(w, "%s ", g.Name)

					fmt.Fprintf(w, "\n  Capabilities: ")
					for _, c := range g.Capabilities {
						fmt.Fprintf(w, "%s ", c)
					}
					fmt.Fprintf(w, "\n  Prefixes:\n")
					for _, p := range g.Prefixes {
						fmt.Fprintf(w, "   %q\n", p)
					}
				}
				return true
			},
		},
	}
}
