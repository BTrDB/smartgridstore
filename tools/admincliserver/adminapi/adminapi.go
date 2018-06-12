package adminapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/BTrDB/btrdb-server/bte"
	"github.com/BTrDB/smartgridstore/acl"
	"github.com/BTrDB/smartgridstore/tools/certutils"
	"github.com/BTrDB/smartgridstore/tools/manifest"
	etcd "github.com/coreos/etcd/clientv3"
	"github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
)

//go:generate protoc -I/usr/local/include -I. -I$GOPATH/src -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis --go_out=plugins=grpc:. adminapi.proto
//go:generate protoc -I/usr/local/include -I. -I$GOPATH/src -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis --grpc-gateway_out=logtostderr=true:.  adminapi.proto
//go:generate protoc -I/usr/local/include -I. -I$GOPATH/src -I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis --swagger_out=logtostderr=true:.  adminapi.proto

func ServeGRPC(ec *etcd.Client, laddr string) {
	cfg, err := certutils.GetAPIConfig(ec)
	if err != nil {
		fmt.Printf("COULD NOT OBTAIN TLS CERTIFICATE\n")
		fmt.Printf("ERR: %v\n", err)
		os.Exit(1)
	}
	if cfg == nil {
		fmt.Printf("TLS setup incomplete, cannot start admin API\n")
		return
	}
	creds := credentials.NewTLS(cfg)
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		panic(err)
	}
	api := &apiProvider{ec: ec}
	grpcServer := grpc.NewServer(grpc.Creds(creds),
		grpc.StreamInterceptor(grpc_auth.StreamServerInterceptor(api.authfunc)),
		grpc.UnaryInterceptor(grpc_auth.UnaryServerInterceptor(api.authfunc)))
	api.s = grpcServer
	RegisterBTrDBAdminServer(grpcServer, api)
	go grpcServer.Serve(l)

	//Insecure server
	il, err := net.Listen("tcp", "127.0.0.1:2250")
	if err != nil {
		panic(err)
	}
	grpcInsecureServer := grpc.NewServer(grpc.StreamInterceptor(grpc_auth.StreamServerInterceptor(api.authfunc)),
		grpc.UnaryInterceptor(grpc_auth.UnaryServerInterceptor(api.authfunc)))
	RegisterBTrDBAdminServer(grpcInsecureServer, api)
	go grpcInsecureServer.Serve(il)
	fmt.Printf("secure/insecure api running\n")
}

func ServeHTTP(ec *etcd.Client, laddr string, svcaddr string) {
	cfg, err := certutils.GetAPIConfig(ec)
	if err != nil {
		fmt.Printf("COULD NOT OBTAIN TLS CERTIFICATE\n")
		fmt.Printf("ERR: %v\n", err)
		os.Exit(1)
	}
	if cfg == nil {
		fmt.Printf("TLS setup incomplete, cannot start admin API\n")
		return
	}

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{
		//Use the operating system root TLS certificates
		//	grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithInsecure(),
	}
	err = RegisterBTrDBAdminHandlerFromEndpoint(context.Background(), mux, "127.0.0.1:2250", opts)
	if err != nil {
		panic(err)
	}

	go func() {
		server := &http.Server{Addr: laddr, Handler: mux, TLSConfig: cfg}
		err := server.ListenAndServeTLS("", "")
		if err != nil {
			panic(err)
		}
	}()

}

type apiProvider struct {
	s  *grpc.Server
	ec *etcd.Client
}

//Copied verbatim from golang HTTP package
func parseBasicAuth(auth string) (username, password string, ok bool) {
	c, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

type TUserObject string

var UserObject TUserObject = "user_object"

func (a *apiProvider) authfunc(ctx context.Context) (context.Context, error) {
	auth, err := grpc_auth.AuthFromMD(ctx, "basic")
	if err != nil {
		return nil, err
	}
	user, pass, ok := parseBasicAuth(auth)
	if !ok {
		fmt.Printf("a\n")
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid basic credentials")
	}
	//Returns false, nil, nil if password is incorrect or user does not exist
	ae := acl.NewACLEngine(acl.DefaultPrefix, a.ec)
	ok, userObj, err := ae.AuthenticateUser(user, pass)
	if err != nil {
		panic(err)
	}
	if !ok {
		fmt.Printf("c")
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid user credentials")
	}

	newCtx := context.WithValue(ctx, UserObject, userObj)
	return newCtx, nil
}

func (a *apiProvider) ManifestAdd(ctx context.Context, p *ManifestAddParams) (*ManifestAddResponse, error) {
	u, ok := ctx.Value(UserObject).(*acl.User)
	if !ok || !u.HasCapability("admin") {
		return &ManifestAddResponse{
			Stat: &Status{
				Code: bte.Unauthorized,
				Msg:  "user does not have 'admin' permissions",
			},
		}, nil
	}
	metadata := make(map[string]string)
	for _, kv := range p.Metadata {
		metadata[kv.Key] = kv.Value
	}
	dev := &manifest.ManifestDevice{Descriptor: p.Deviceid, Metadata: metadata, Streams: make(map[string]*manifest.ManifestDeviceStream)}
	success, err := manifest.UpsertManifestDeviceAtomically(ctx, a.ec, dev)
	if err != nil {
		return &ManifestAddResponse{
			Stat: &Status{
				Code: bte.ManifestError,
				Msg:  err.Error(),
			},
		}, nil
	}
	if !success {
		return &ManifestAddResponse{
			Stat: &Status{
				Code: bte.ManifestDeviceDuplicated,
				Msg:  err.Error(),
			},
		}, nil
	}
	return &ManifestAddResponse{}, nil
}

func (a *apiProvider) ManifestDel(ctx context.Context, p *ManifestDelParams) (*ManifestDelResponse, error) {
	u, ok := ctx.Value(UserObject).(*acl.User)
	if !ok || !u.HasCapability("admin") {
		return &ManifestDelResponse{
			Stat: &Status{
				Code: bte.Unauthorized,
				Msg:  "user does not have 'admin' permissions",
			},
		}, nil
	}
	err := manifest.DeleteManifestDevice(ctx, a.ec, p.Deviceid)
	if err != nil {
		return &ManifestDelResponse{
			Stat: &Status{
				Code: bte.ManifestError,
				Msg:  err.Error(),
			},
		}, nil
	}
	return &ManifestDelResponse{}, nil
}

func (a *apiProvider) ManifestDelPrefix(ctx context.Context, p *ManifestDelPrefixParams) (*ManifestDelPrefixResponse, error) {
	u, ok := ctx.Value(UserObject).(*acl.User)
	if !ok || !u.HasCapability("admin") {
		return &ManifestDelPrefixResponse{
			Stat: &Status{
				Code: bte.Unauthorized,
				Msg:  "user does not have 'admin' permissions",
			},
		}, nil
	}
	n, err := manifest.DeleteMultipleManifestDevices(ctx, a.ec, p.Deviceidprefix)
	if err != nil {
		return &ManifestDelPrefixResponse{
			Stat: &Status{
				Code: bte.ManifestError,
				Msg:  err.Error(),
			},
		}, nil
	}
	return &ManifestDelPrefixResponse{
		Numdeleted: uint32(n),
	}, nil
}

func (a *apiProvider) ManifestLsDevs(ctx context.Context, p *ManifestLsDevsParams) (*ManifestLsDevsResponse, error) {
	u, ok := ctx.Value(UserObject).(*acl.User)
	if !ok || !u.HasCapability("admin") {
		fmt.Printf("XX\n")
		return &ManifestLsDevsResponse{
			Stat: &Status{
				Code: bte.Unauthorized,
				Msg:  "user does not have 'admin' permissions",
			},
		}, nil
	}
	devs, err := manifest.RetrieveMultipleManifestDevices(ctx, a.ec, p.Deviceidprefix)
	if err != nil {
		return &ManifestLsDevsResponse{
			Stat: &Status{
				Code: bte.ManifestError,
				Msg:  err.Error(),
			},
		}, nil
	}
	rv := &ManifestLsDevsResponse{}
	for _, dev := range devs {
		d := &ManifestDevice{Deviceid: dev.Descriptor}
		for k, v := range dev.Metadata {
			d.Metadata = append(d.Metadata, &MetaKeyValue{Key: k, Value: v})
		}
		rv.Devices = append(rv.Devices, d)
	}
	return rv, nil
}
