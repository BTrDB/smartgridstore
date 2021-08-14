// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

import (
	"fmt"
	"html/template"
	"os"
	"path"
)

type ManifestEntry struct {
	Needed   func(s *SiteConfig) bool
	Filename string
	Generate func(dir string, s *SiteConfig) error
}

var Manifest []*ManifestEntry = []*ManifestEntry{
	&ManifestEntry{Yep, "READMEFIRST.md", TemplateProcess("readme.md", "")},
	&ManifestEntry{Yep, "adminconsole.deployment.yaml", TemplateProcess("adminconsole.deployment.yaml", "core")},
	&ManifestEntry{Yep, "apifrontend.deployment.yaml", TemplateProcess("apifrontend.deployment.yaml", "core")},
	&ManifestEntry{Yep, "ensuredb.job.yaml", TemplateProcess("ensuredb.job.yaml", "core")},
	&ManifestEntry{Yep, "btrdb.statefulset.yaml", TemplateProcess("btrdb.statefulset.yaml", "core")},
	&ManifestEntry{Yep, "create_admin_key.sh", TemplateProcess("create_admin_key.sh", "core")},
	&ManifestEntry{Yep, "c37ingress.deployment.yaml", TemplateProcess("c37ingress.deployment.yaml", "ingress")},
	&ManifestEntry{Yep, "gepingress.deployment.yaml", TemplateProcess("gepingress.deployment.yaml", "ingress")},
	&ManifestEntry{Yep, "ingester.deployment.yaml", TemplateProcess("ingester.deployment.yaml", "ingress")},
	&ManifestEntry{Yep, "receiver.deployment.yaml", TemplateProcess("receiver.deployment.yaml", "ingress")},
	&ManifestEntry{Yep, "etcd-cluster.daemonset.yaml", TemplateProcess("etcd-cluster.daemonset.yaml", "core")},
	&ManifestEntry{Yep, "pmu2btrdb.deployment.yaml", TemplateProcess("pmu2btrdb.deployment.yaml", "ingress")},
	&ManifestEntry{Yep, "mrplotter.deployment.yaml", TemplateProcess("mrplotter.deployment.yaml", "core")},
	&ManifestEntry{Yep, "secret_ceph_keyring.sh", TemplateProcess("secret_ceph_keyring.sh", "core")},
}

func Yep(s *SiteConfig) bool {
	return true
}
func TemplateProcess(filename string, templatedir string) func(dir string, s *SiteConfig) error {
	return func(dir string, s *SiteConfig) error {
		err := os.MkdirAll(path.Join(dir, templatedir), 0777)
		if err != nil {
			return err
		}
		of, err := os.Create(path.Join(dir, templatedir, filename))
		if err != nil {
			return err
		}
		src, err := Asset(filename)
		if err != nil {
			return fmt.Errorf("internal error: template %q not found (%v)", filename, err)
		}
		templ, err := template.New("root").Parse(string(src))
		if err != nil {
			return fmt.Errorf("internal error: template %q malformed (%v)", filename, err)
		}
		err = templ.Execute(of, s)
		return err
	}
}
