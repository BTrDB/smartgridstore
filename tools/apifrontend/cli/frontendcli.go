// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package cli

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/BTrDB/smartgridstore/admincli"
	etcd "github.com/coreos/etcd/clientv3"
)

var etcdConn *etcd.Client

func NewFrontendModule(c *etcd.Client) admincli.CLIModule {
	etcdConn = c
	return &admincli.GenericCLIModule{
		MName:  "api",
		MHint:  "manage settings for the API frontend",
		MUsage: "",
		MChildren: []admincli.CLIModule{
			&admincli.GenericCLIModule{
				MName:     "setcertsrc",
				MHint:     "sets the method by which the certificate is obtained",
				MUsage:    " autocert/hardcoded/disabled",
				MRun:      setcertsrc,
				MRunnable: true,
			},
			&admincli.GenericCLIModule{
				MName:     "getcertsrc",
				MHint:     "gets the method by which the certificate is obtained",
				MUsage:    "",
				MRun:      getcertsrc,
				MRunnable: true,
			},
			&admincli.GenericCLIModule{
				MName:     "autocert",
				MHint:     "configure autocert",
				MUsage:    "",
				MRunnable: false,
				MChildren: []admincli.CLIModule{
					&admincli.GenericCLIModule{
						MName:     "sethost",
						MHint:     "sets the hostname for autocert",
						MUsage:    "",
						MRun:      sethost,
						MRunnable: true,
					},
					&admincli.GenericCLIModule{
						MName:     "setemail",
						MHint:     "sets the email address for autocert",
						MUsage:    "",
						MRun:      setemail,
						MRunnable: true,
					},
					&admincli.GenericCLIModule{
						MName:     "show",
						MHint:     "shows autocert information",
						MUsage:    "",
						MRun:      show,
						MRunnable: true,
					},
				},
			},
			&admincli.GenericCLIModule{
				MName:     "sethardcoded",
				MHint:     "sets the certificate to use when certsrc is hardcoded",
				MUsage:    " cert key",
				MRunnable: true,
				MRun:      sethardcoded,
			},
			&admincli.GenericCLIModule{
				MName:     "gethardcoded",
				MHint:     "gets the certificate used when certsrc is hardcoded",
				MUsage:    "",
				MRunnable: true,
				MRun:      gethardcoded,
			},
		},
	}
}

func setkey(c *etcd.Client, keysuffix string, value string) error {
	_, err := c.Put(context.Background(), "api/"+keysuffix, value)
	return err
}

func getkey(c *etcd.Client, keysuffix string) (string, error) {
	resp, err := c.Get(context.Background(), "api/"+keysuffix)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", err
	}
	return string(resp.Kvs[0].Value), nil
}

func gethardcoded(ctx context.Context, out io.Writer, args ...string) bool {
	privrv, err := getkey(etcdConn, "hardcoded_priv")
	if err != nil {
		fmt.Fprintf(out, "error: %v\n", err)
		return true
	}
	pubrv, err := getkey(etcdConn, "hardcoded_pub")
	if err != nil {
		fmt.Fprintf(out, "error: %v\n", err)
		return true
	}
	pubrv = base64.StdEncoding.EncodeToString([]byte(pubrv))
	privrv = base64.StdEncoding.EncodeToString([]byte(privrv))
	fmt.Fprintf(out, "public certificate:\n%s\nprivate key:\n%s\n", pubrv, privrv)
	return true
}
func sethardcoded(ctx context.Context, out io.Writer, args ...string) bool {
	if len(args) != 2 {
		return false
	}
	pub, err := base64.StdEncoding.DecodeString(args[0])
	if err != nil {
		fmt.Fprintf(out, "the public certificate (arg 1) is invalid base64\n")
		return true
	}
	priv, err := base64.StdEncoding.DecodeString(args[1])
	if err != nil {
		fmt.Fprintf(out, "the private key (arg 2) is invalid base64\n")
		return true
	}
	err = setkey(etcdConn, "hardcoded_priv", string(priv))
	if err != nil {
		fmt.Fprintf(out, "unexpected error: %v\n", err)
		return true
	}
	err = setkey(etcdConn, "hardcoded_pub", string(pub))
	if err != nil {
		fmt.Fprintf(out, "unexpected error: %v\n", err)
		return true
	}
	return true
}
func show(ctx context.Context, out io.Writer, args ...string) bool {
	host, err := getkey(etcdConn, "host")
	if err != nil {
		fmt.Fprintf(out, "unexpected error: %v\n", err)
		return true
	}
	email, err := getkey(etcdConn, "email")
	if err != nil {
		fmt.Fprintf(out, "unexpected error: %v\n", err)
		return true
	}
	fmt.Fprintf(out, "hostname: %v\nemail: %v\n", host, email)
	return true
}

func setcertsrc(ctx context.Context, out io.Writer, args ...string) bool {
	if len(args) != 1 {
		return false
	}
	if args[0] != "autocert" && args[0] != "hardcoded" && args[0] != "disabled" {
		fmt.Fprintf(out, "src must be one of autocert, hardcoded or disabled\n")
		return true
	}
	err := setkey(etcdConn, "certsrc", args[0])
	if err != nil {
		fmt.Fprintf(out, "unexpected error: %v\n", err)
		return true
	}
	return true
}
func getcertsrc(ctx context.Context, out io.Writer, args ...string) bool {
	certsrc, err := getkey(etcdConn, "certsrc")
	if err != nil {
		fmt.Fprintf(out, "unexpected error: %v\n", err)
		return true
	}
	if certsrc == "" {
		certsrc = "disabled"
	}
	fmt.Fprintf(out, "certsrc: %s\n", certsrc)
	return true
}
func setemail(ctx context.Context, out io.Writer, args ...string) bool {
	if len(args) != 1 {
		return false
	}
	err := setkey(etcdConn, "email", args[0])
	if err != nil {
		fmt.Fprintf(out, "unexpected error: %v\n", err)
		return true
	}
	return true
}
func sethost(ctx context.Context, out io.Writer, args ...string) bool {
	if len(args) != 1 {
		return false
	}
	err := setkey(etcdConn, "host", args[0])
	if err != nil {
		fmt.Fprintf(out, "unexpected error: %v\n", err)
		return true
	}
	return true
}
