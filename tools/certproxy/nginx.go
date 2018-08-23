package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"strings"
)

type Route struct {
	Upstream string
	Path     string
}

type TemplateParameters struct {
	Upstreams   map[string]Route
	Servername  string
	Certificate string
	Certkey     string
}

var nginxGlobalConfig = `
daemon off;
error_log /dev/stdout info;
`
var nginxConfigTemplate = `

{{ range $key, $value := .Upstreams }}
upstream {{ $key }} {
    server {{ $value.Upstream }};
}
{{ end }}

server {
	listen 80 default_server;
	listen [::]:80 default_server;
	server_name _;
	return 301 https://$host$request_uri;
}

server {
    listen              443 ssl; # 'ssl' parameter tells NGINX to decrypt the traffic
    server_name         {{ .Servername }};
    ssl_certificate     {{ .Certificate }};
    ssl_certificate_key {{ .Certkey }};
		access_log /dev/stdout;

		# for certbot renew challenge
    location /.well-known {
      root /var/www/;
    }

		{{ range $key, $value := .Upstreams }}
		location {{ $value.Path }} {
			proxy_pass http://{{ $key }};
		}
		{{ end }}
}
`

func writeNginxConfig(cd *CertDetails) {
	t := template.New("main")
	t, err := t.Parse(nginxConfigTemplate)
	if err != nil {
		panic(err)
	}
	routenum := 0
	routes := make(map[string]Route)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		key := strings.ToUpper(parts[0])
		val := parts[1]
		if !strings.HasPrefix(key, "ROUTE_") {
			continue
		}
		valparts := strings.SplitN(val, "->", 2)
		if len(valparts) != 2 {
			fmt.Printf("malformed ROUTE directive %q\n", env)
			os.Exit(1)
		}
		r := Route{
			Path:     valparts[0],
			Upstream: valparts[1],
		}
		routes[fmt.Sprintf("upstream%d", routenum)] = r
		routenum++
	}
	buf := bytes.Buffer{}
	params := &TemplateParameters{}
	params.Servername = cd.Domain
	params.Upstreams = routes
	if !cd.IsAutocert {
		err := ioutil.WriteFile("/etc/nginx/cert.pem", cd.HardcodedCert, 0700)
		if err != nil {
			fmt.Printf("could not write out certificate: %v\n", err)
			os.Exit(1)
		}
		err = ioutil.WriteFile("/etc/nginx/key.pem", cd.HardcodedKey, 0700)
		if err != nil {
			fmt.Printf("could not write out key: %v\n", err)
			os.Exit(1)
		}
		params.Certificate = "/etc/nginx/cert.pem"
		params.Certkey = "/etc/nginx/key.pem"
	} else {
		params.Certificate = "/etc/letsencrypt/live/" + cd.Domain + "/fullchain.pem"
		params.Certkey = "/etc/letsencrypt/live/" + cd.Domain + "/privkey.pem"
	}
	err = t.Execute(&buf, params)
	if err != nil {
		fmt.Printf("could not build nginx config: %v\n", err)
		os.Exit(1)
	}
	gconf, err := os.OpenFile("/etc/nginx/nginx.conf", os.O_WRONLY|os.O_APPEND, 0777)
	if err != nil {
		panic(err)
	}
	_, err = gconf.WriteString(nginxGlobalConfig)
	if err != nil {
		panic(err)
	}
	gconf.Close()

	err = ioutil.WriteFile("/etc/nginx/sites-available/default", buf.Bytes(), 0777)
	if err != nil {
		fmt.Printf("could not write nginx config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[global] nginx config written\n")
}
