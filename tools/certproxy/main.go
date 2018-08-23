package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/BTrDB/mr-plotter/keys"
	"github.com/BTrDB/smartgridstore/tools"
	etcd "github.com/coreos/etcd/clientv3"
)

var etcdConn *etcd.Client

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		fmt.Printf("%d.%d.%d\n", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch)
		os.Exit(0)
	}
	fmt.Printf("[global] Booting certificate proxy %d.%d.%d\n", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch)

	var etcdEndpoint string = os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "localhost:2379"
		log.Printf("ETCD_ENDPOINT is not set; using %s", etcdEndpoint)
	}

	var etcdConfig etcd.Config = etcd.Config{Endpoints: []string{etcdEndpoint}}

	log.Println("[global] Connecting to etcd...")
	var err error
	etcdConn, err = etcd.New(etcdConfig)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer etcdConn.Close()

	fmt.Printf("[global] Loading certificate state...\n")
	cd := loadCertDetails()
	if cd.IsAutocert {
		skipInitialConfig := loadEtcDirectory(cd)
		if !skipInitialConfig {
			fmt.Printf("[global] performing initial certificate acquisition\n")
			certbotAcquire(cd)
		}
	}
	go startNginx(cd)
	time.Sleep(1 * time.Minute)
	if cd.IsAutocert {
		go renewalLoop(cd)
	}
	for {
		time.Sleep(1 * time.Second)
	}
}

func renewalLoop(cd *CertDetails) {
	for {
		{
			fmt.Printf("[global] RENEWING CERTIFICATE\n")
			cmd := exec.Command("certbot", "renew", "--webroot", "--webroot-path", "/var/www")
			stderr, err := cmd.StderrPipe()
			if err != nil {
				panic(err)
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				panic(err)
			}
			go printPrefix(stderr, "certbot_renew:stderr")
			go printPrefix(stdout, "certbot_renew:stdout")
			err = cmd.Run()
			if err != nil {
				fmt.Printf("CERTBOT_RENEW RUN FAILURE: %v\n", err)
				os.Exit(1)
			}
		}
		fmt.Printf("[global] Certificate renewal successful, reloading nginx\n")
		{
			cmd := exec.Command("/usr/sbin/nginx", "-s", "reload")
			stderr, err := cmd.StderrPipe()
			if err != nil {
				panic(err)
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				panic(err)
			}
			go printPrefix(stderr, "nginx_reload:stderr")
			go printPrefix(stdout, "nginx_reload:stdout")
			err = cmd.Run()
			if err != nil {
				fmt.Printf("NGINX_RELOAD RUN FAILURE: %v\n", err)
				os.Exit(1)
			}
		}
		saveEtcDirectory()
		saveAutocertCache("/etc/letsencrypt/live/"+cd.Domain+"/privkey.pem",
			"/etc/letsencrypt/live/"+cd.Domain+"/fullchain.pem",
			cd.Domain)
		time.Sleep(24 * time.Hour)
	}
}
func printPrefix(pipe io.ReadCloser, prefix string) {
	br := bufio.NewReader(pipe)
	for {
		ln, _, err := br.ReadLine()
		if err != nil {
			fmt.Printf("[%s] ERROR: %v\n", prefix, err)
			return
		}
		fmt.Printf("[%s] %s\n", prefix, ln)
	}
}
func startNginx(cd *CertDetails) {
	writeNginxConfig(cd)
	cmd := exec.Command("/usr/sbin/nginx")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	go printPrefix(stderr, "nginx:stderr")
	go printPrefix(stdout, "nginx:stdout")
	err = cmd.Run()
	if err != nil {
		fmt.Printf("NGINX RUN FAILURE: %v\n", err)
		os.Exit(1)
	}
}

const statePath = "certproxy/state/"
const configDir = "/etc/letsencrypt/"

type CertDetails struct {
	IsAutocert    bool
	Domain        string
	Email         string
	HardcodedKey  []byte
	HardcodedCert []byte
}

func loadCertDetails() *CertDetails {
	ctx := context.Background()
	certsrc, err := keys.GetCertificateSource(ctx, etcdConn)
	if err != nil {
		panic(err)
	}
	if certsrc == "" {
		fmt.Printf("certificate source not set in admin console\n")
		fmt.Printf("aborting...")
		os.Exit(1)
	}
	rv := &CertDetails{
		IsAutocert: certsrc != "hardcoded",
	}
	domain, err := keys.GetAutocertHostname(ctx, etcdConn)
	if err != nil {
		panic(err)
	}
	rv.Domain = domain
	if domain == "" {
		fmt.Printf("autocert domain must be set in the admin console\n")
		os.Exit(1)
	}
	if certsrc == "autocert" {
		email, err := keys.GetAutocertEmail(ctx, etcdConn)
		if err != nil {
			panic(err)
		}
		rv.Email = email
	} else {
		hardcoded, err := keys.RetrieveHardcodedTLSCertificate(ctx, etcdConn)
		if err != nil {
			panic(err)
		}
		if hardcoded == nil || len(hardcoded.Cert) == 0 || len(hardcoded.Key) == 0 {
			fmt.Printf("certificate source set to hardcoded, but the hardcoded certificate is missing\n")
			os.Exit(1)
		}
		rv.HardcodedCert = hardcoded.Cert
		rv.HardcodedKey = hardcoded.Key
	}
	return rv
}
func certbotAcquire(cd *CertDetails) {
	//certbot certonly -d bunker.cal-sdb.org --staging --standalone --agree-tos -m testing@steelcode.com -n
	cmd := exec.Command("certbot", "certonly", "-d", cd.Domain, "--standalone", "--agree-tos", "-m", cd.Email, "-n")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	go printPrefix(stderr, "certbot_initial:stderr")
	go printPrefix(stdout, "certbot_initial:stdout")
	err = cmd.Run()
	if err != nil {
		fmt.Printf("CERTBOT ACQUIRE RUN FAILURE: %v\n", err)
		os.Exit(1)
	}
	saveEtcDirectory()
	saveAutocertCache("/etc/letsencrypt/live/"+cd.Domain+"/privkey.pem",
		"/etc/letsencrypt/live/"+cd.Domain+"/fullchain.pem",
		cd.Domain)
}
func loadEtcDirectory(cd *CertDetails) bool {
	if !cd.IsAutocert {
		panic("we should not load directory if not in autocert mode\n")
	}
	domain := cd.Domain
	tofind := "live/" + domain + "/privkey.pem"
	found := false
	//Load the certificates from etcd
	resp, err := etcdConn.Get(context.Background(), statePath, etcd.WithPrefix())
	if err != nil {
		fmt.Printf("failed to query state from etcd: %v\n", err)
		os.Exit(1)
	}
	for _, kv := range resp.Kvs {
		suffix := strings.TrimPrefix(string(kv.Key), statePath)
		if suffix == tofind {
			found = true
		}
		kpath := configDir + suffix
		content := kv.Value
		d := path.Dir(kpath)
		err := os.MkdirAll(d, 0777)
		if err != nil {
			fmt.Printf("failed to create directory: %v\n", err)
			os.Exit(1)
		}
		err = ioutil.WriteFile(kpath, content, 0555)
		if err != nil {
			fmt.Printf("failed to write file: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("finished loading etcd directory\n")
	return found
}
func walk(dir string, files map[string][]byte) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("file walking error: %v\n", err)
			os.Exit(1)
		}
		if info.IsDir() {
			//do nothing, we will traverse into the directory anyway
		} else {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				fmt.Printf("file read error: %v\n", err)
				os.Exit(1)
			}
			files[path] = content
		}
		return nil
	})
	if err != nil {
		fmt.Printf("file walking error: %v\n", err)
		os.Exit(1)
	}
}
func saveAutocertCache(keyfile string, certfile string, domain string) {
	key, err := ioutil.ReadFile(keyfile)
	if err != nil {
		fmt.Printf("could not read key file: %v\n", err)
		os.Exit(1)
	}
	cert, err := ioutil.ReadFile(certfile)
	if err != nil {
		fmt.Printf("could not read certificate file: %v\n", err)
		os.Exit(1)
	}
	concat := string(key) + string(cert)
	_, err = etcdConn.Put(context.Background(), "mrplotter/keys/autocert_cache/"+domain, concat)
	if err != nil {
		fmt.Printf("failed to write back autocert cache: %v\n", err)
		os.Exit(1)
	}
}
func saveEtcDirectory() {
	files := make(map[string][]byte)
	walk(configDir, files)

	ops := []etcd.Op{}
	for file, content := range files {
		suffix := strings.TrimPrefix(file, configDir)
		key := statePath + suffix
		ops = append(ops, etcd.OpPut(key, string(content)))
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err := etcdConn.Txn(ctx).
		Then(ops...).
		Commit()
	cancel()
	if err != nil {
		fmt.Printf("failed to write back etcd state: %v\n", err)
		os.Exit(1)
	}
}
