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
	&ManifestEntry{Yep, "adminconsole.deployment.yaml", TemplateProcess("adminconsole.deployment.yaml")},
	&ManifestEntry{Yep, "createdb.job.yaml", TemplateProcess("createdb.job.yaml")},
	&ManifestEntry{Yep, "btrdb.statefulset.yaml", TemplateProcess("btrdb.statefulset.yaml")},
	&ManifestEntry{Yep, "create_admin_key.sh", TemplateProcess("create_admin_key.sh")},
	&ManifestEntry{Yep, "etcd.statefulset.yaml", TemplateProcess("etcd.statefulset.yaml")},
	&ManifestEntry{Yep, "ingester.deployment.yaml", TemplateProcess("ingester.deployment.yaml")},
	&ManifestEntry{Yep, "mrplotter.deployment.yaml", TemplateProcess("mrplotter.deployment.yaml")},
	&ManifestEntry{Yep, "receiver.deployment.yaml", TemplateProcess("receiver.deployment.yaml")},
	&ManifestEntry{Yep, "pmu2btrdb.deployment.yaml", TemplateProcess("pmu2btrdb.deployment.yaml")},
}

func Yep(s *SiteConfig) bool {
	return true
}
func TemplateProcess(filename string) func(dir string, s *SiteConfig) error {
	return func(dir string, s *SiteConfig) error {
		of, err := os.Create(path.Join(dir, filename))
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
