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
	&ManifestEntry{Yep, "createdb.job.yaml", TemplateProcess("createdb.job.yaml", "core")},
	&ManifestEntry{Yep, "ensuredb.job.yaml", TemplateProcess("ensuredb.job.yaml", "core")},
	&ManifestEntry{Yep, "btrdb.statefulset.yaml", TemplateProcess("btrdb.statefulset.yaml", "core")},
	&ManifestEntry{Yep, "create_admin_key.sh", TemplateProcess("create_admin_key.sh", "core")},
	&ManifestEntry{Yep, "c37ingress.deployment.yaml", TemplateProcess("c37ingress.deployment.yaml", "ingress")},
	&ManifestEntry{Yep, "ingester.deployment.yaml", TemplateProcess("ingester.deployment.yaml", "ingress")},
	&ManifestEntry{Yep, "receiver.deployment.yaml", TemplateProcess("receiver.deployment.yaml", "ingress")},
	&ManifestEntry{Yep, "pmu2btrdb.deployment.yaml", TemplateProcess("pmu2btrdb.deployment.yaml", "ingress")},
	&ManifestEntry{Yep, "etcd.clusterrole.yaml", TemplateProcess("etcd.clusterrole.yaml", "global")},
	&ManifestEntry{Yep, "mrplotter.deployment.yaml", TemplateProcess("mrplotter.deployment.yaml", "core")},
	&ManifestEntry{Yep, "etcd.clusterrolebinding.yaml", TemplateProcess("etcd.clusterrolebinding.yaml", "core")},
	&ManifestEntry{Yep, "etcd.serviceaccount.yaml", TemplateProcess("etcd.serviceaccount.yaml", "core")},
	&ManifestEntry{Yep, "etcd.cluster.yaml", TemplateProcess("etcd.cluster.yaml", "core")},
	&ManifestEntry{Yep, "etcd-operator.deployment.yaml", TemplateProcess("etcd-operator.deployment.yaml", "core")},
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
