# CertProxy

CertProxy is a TLS reverse proxy that is integrated with the autocert settings
that are configured in the admin console. It allows arbitrary services to be
served over HTTP with TLS added at the border.

The container will automatically apply for and renew LetsEncrypt certificates
using the hostname and email address configured in the admin console. It can
also be used with hardcoded certificates (set via certutils/setcert).

PLEASE NOTE: even if hardcoded certificates are being used, it is necessary to
fill in the hostname in the autocert configuration in adminconsole so that the
server knows what servername to advertise.

## Configuration

Configuration is done via environment variables. An arbitrary number of routing
rules may be defined as follows:

```
ROUTE_MYSERVICE=/the/url/prefix->the.internal.host:port
ROUTE_EXAMPLE=/example->123.123.123.1:8080
```

If there are multiple routes that overlap, the longest match will be applied. For
example if the routes configured are:

```
ROUTE_A=/->plotter:80
ROUTE_B=/adminapi->adminapi:80
```

Then everything starting with `adminapi` will be longest-matched to the adminapi
upstream servie but every other uri will be forwarded to the plotter service.
