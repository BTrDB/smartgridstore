package acl

import (
	"context"
	"fmt"
	"io"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/BTrDB/smartgridstore/admincli"
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
	aclEngine := NewACLEngine(c)
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
			users,
		},
	}
}

func adduser(curUser string, e *ACLEngine, ctx context.Context, w io.Writer, args ...string) bool {
	if len(args) != 2 {
		return false
	}
	if err := e.Test(curUser, PCreateUser); err != nil {
		fmt.Fprintf(w, "failed: %v\n", err)
		return true
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
	if err := e.Test(curUser, PDeleteUser); err != nil {
		fmt.Fprintf(w, "failed: %v\n", err)
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
				if su.loggedInUser != su.username {
					if err := su.e.Test(su.loggedInUser, PChangePassword); err != nil {
						fmt.Fprintf(w, "failed: %v\n", err)
						return true
					}
				}
				if err := su.e.SetPassword(su.username, args[0]); err != nil {
					fmt.Fprintf(w, "failed: %v\n", err)
					return true
				}
				fmt.Fprintf(w, "password changed\n")
				return true
			},
		},
		&admincli.GenericCLIModule{
			MName:     "allow",
			MHint:     "add user permissions",
			MUsage:    " key[=value] [key[=value]] ...",
			MRunnable: true,
			MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
				fmt.Fprintf(w, "not implemented")
				return true
			},
		},
		&admincli.GenericCLIModule{
			MName:     "revoke",
			MHint:     "revoke user permissions",
			MUsage:    " key [key] [key] ...",
			MRunnable: true,
			MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
				fmt.Fprintf(w, "not implemented")
				return true
			},
		},
		&admincli.GenericCLIModule{
			MName:     "describe",
			MHint:     "print user info",
			MUsage:    "",
			MRunnable: true,
			MRun: func(ctx context.Context, w io.Writer, args ...string) bool {
				fmt.Fprintf(w, "not implemented")
				return true
			},
		},
	}
}
