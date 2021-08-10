// Copyright (c) 2021 Michael Andersen
// Copyright (c) 2021 Regents of the University Of California
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"sync"

	"os"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/clientv3"

	btrdb "gopkg.in/BTrDB/btrdb.v4"

	"github.com/BTrDB/smartgridstore/acl"
	"github.com/BTrDB/smartgridstore/tools"
	"github.com/BTrDB/smartgridstore/tools/certutils"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	logging "github.com/op/go-logging"
	"github.com/pborman/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	pb "gopkg.in/BTrDB/btrdb.v4/grpcinterface"
)

const MajorVersion = tools.VersionMajor
const MinorVersion = tools.VersionMinor

type CK string

const UserKey CK = "user"

type apiProvider struct {
	s          *grpc.Server
	downstream *btrdb.BTrDB
	ae         *acl.ACLEngine
	colUUcache map[[16]byte]string
	colUUmu    sync.Mutex
	secure     bool
}

var logger *logging.Logger

func init() {
	logger = logging.MustGetLogger("log")
}

func (a *apiProvider) authfunc(ctx context.Context) (context.Context, error) {
	var userObj *acl.User
	auth, err := grpc_auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		if grpc.Code(err) == codes.Unauthenticated {
			userObj, err = a.ae.GetPublicUser()
			if err != nil {
				panic(err)
			}
		} else {
			return nil, err
		}
	}

	if auth != "" {
		//Returns false, nil, nil if password is incorrect or user does not exist
		var ok bool
		ok, userObj, err = a.ae.AuthenticateUserByKey(auth)
		if !ok {
			return nil, grpc.Errorf(codes.Unauthenticated, "invalid api key")
		}
	}
	newCtx := context.WithValue(ctx, UserKey, userObj)
	return newCtx, nil
}

//go:generate ./genswag.py
//go:generate go-bindata -pkg main swag/...
func serveSwagger(mux *http.ServeMux) {
	mime.AddExtensionType(".svg", "image/svg+xml")

	// Expose files in third_party/swagger-ui/ on <host>/swagger-ui
	fileServer := http.FileServer(&assetfs.AssetFS{
		Asset:     Asset,
		AssetDir:  AssetDir,
		AssetInfo: AssetInfo,
		Prefix:    "swag",
	})
	prefix := "/swag/"
	mux.Handle(prefix, http.StripPrefix(prefix, fileServer))
}

type GRPCInterface interface {
	InitiateShutdown() chan struct{}
}

//Copied verbatim from golang HTTP package
func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
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

func (a *apiProvider) writeEndpoint(ctx context.Context, uu uuid.UUID) (*btrdb.Endpoint, error) {
	return a.downstream.EndpointFor(ctx, uu)
}
func (a *apiProvider) readEndpoint(ctx context.Context, uu uuid.UUID) (*btrdb.Endpoint, error) {
	return a.downstream.ReadEndpointFor(ctx, uu)
}
func (a *apiProvider) anyEndpoint(ctx context.Context) (*btrdb.Endpoint, error) {
	return a.downstream.GetAnyEndpoint(ctx)
}

// func (a *apiProvider) getUser(ctx context.Context) (*User, error) {
// 	return nil, nil
// }
func (a *apiProvider) checkPermissionsByUUID(ctx context.Context, uu uuid.UUID, cap ...string) error {
	a.colUUmu.Lock()
	col, ok := a.colUUcache[uu.Array()]
	a.colUUmu.Unlock()
	if !ok {
		var err error
		s := a.downstream.StreamFromUUID(uu)
		col, err = s.Collection(ctx)
		if err != nil {
			if e := btrdb.ToCodedError(err); e != nil && e.Code == 404 {
				return grpc.Errorf(codes.PermissionDenied, "user does not have permission on this stream")
			}
			return err
		}
		a.colUUmu.Lock()
		a.colUUcache[uu.Array()] = col
		a.colUUmu.Unlock()
	}
	return a.checkPermissionsByCollection(ctx, col, cap...)
}
func (a *apiProvider) checkPermissionsByCollection(ctx context.Context, collection string, cap ...string) error {
	u, ok := ctx.Value(UserKey).(*acl.User)
	if !ok {
		return grpc.Errorf(codes.PermissionDenied, "could not resolve user")
	}
	for _, cp := range cap {
		if !u.HasCapabilityOnPrefix(cp, collection) {
			return grpc.Errorf(codes.PermissionDenied, "user does not have permission %q on %q", cp, collection)
		}
	}
	return nil
}

func ProxyGRPCSecure(laddr string) *tls.Config {
	etcdEndpoint := os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "http://etcd:2379"
	}
	etcdClient, err := etcd.New(etcd.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: 5 * time.Second})
	if err != nil {
		fmt.Printf("Could not connect to etcd: %v\n", err)
		os.Exit(1)
	}

	cfg, err := certutils.GetAPIConfig(etcdClient)
	if cfg == nil {
		fmt.Printf("TLS config is incomplete (%v), disabling secure endpoints\n", err)
		return nil
	}

	creds := credentials.NewTLS(cfg)
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	downstream, err := btrdb.Connect(ctx, btrdb.EndpointsFromEnv()...)
	cancel()
	if err != nil {
		panic(err)
	}
	api := &apiProvider{downstream: downstream, colUUcache: make(map[[16]byte]string)}
	api.secure = true
	ae := acl.NewACLEngine(acl.DefaultPrefix, etcdClient)
	api.ae = ae
	//--
	grpcServer := grpc.NewServer(grpc.Creds(creds),
		grpc.StreamInterceptor(grpc_auth.StreamServerInterceptor(api.authfunc)),
		grpc.UnaryInterceptor(grpc_auth.UnaryServerInterceptor(api.authfunc)))
	//--
	api.s = grpcServer

	pb.RegisterBTrDBServer(grpcServer, api)
	go grpcServer.Serve(l)
	return cfg
}

func ProxyGRPC(laddr string) GRPCInterface {
	etcdEndpoint := os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "http://etcd:2379"
	}
	etcdClient, err := etcd.New(etcd.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: 5 * time.Second})
	if err != nil {
		fmt.Printf("Could not connect to etcd: %v\n", err)
		os.Exit(1)
	}
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	downstream, err := btrdb.Connect(ctx, btrdb.EndpointsFromEnv()...)
	cancel()
	if err != nil {
		panic(err)
	}
	api := &apiProvider{downstream: downstream, colUUcache: make(map[[16]byte]string)}
	api.secure = false
	ae := acl.NewACLEngine(acl.DefaultPrefix, etcdClient)
	api.ae = ae
	//--
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpc_auth.StreamServerInterceptor(api.authfunc)),
		grpc.UnaryInterceptor(grpc_auth.UnaryServerInterceptor(api.authfunc)))
	//--
	api.s = grpcServer
	pb.RegisterBTrDBServer(grpcServer, api)
	go grpcServer.Serve(l)
	return api
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		fmt.Printf("%d.%d.%d\n", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch)
		os.Exit(0)
	}

	disable_insecure := strings.ToLower(os.Getenv("DISABLE_INSECURE")) == "yes"
	insecure_listen := "0.0.0.0:4410"
	if disable_insecure {
		insecure_listen = "127.0.0.1:4410"
	}
	ProxyGRPC(insecure_listen)
	tlsconfig := ProxyGRPCSecure("0.0.0.0:4411")

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/swagger.json", func(w http.ResponseWriter, req *http.Request) {
		io.Copy(w, strings.NewReader(SwaggerJSON))
	})
	mux.HandleFunc("/v4/query", queryhandler)
	gwmux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	err := pb.RegisterBTrDBHandlerFromEndpoint(ctx, gwmux, "127.0.0.1:4410", opts)
	if err != nil {
		panic(err)
	}
	mux.Handle("/", gwmux)
	serveSwagger(mux)

	if !disable_insecure {
		go func() {
			err := http.ListenAndServe(":9000", mux)
			if err != nil {
				panic(err)
			}
		}()
	}
	if tlsconfig != nil {
		fmt.Printf("starting secure http\n")
		go func() {
			server := &http.Server{Addr: ":9001", Handler: mux, TLSConfig: tlsconfig}
			err := server.ListenAndServeTLS("", "")
			if err != nil {
				panic(err)
			}
		}()
	} else {
		fmt.Printf("skipping secure http\n")
	}
	for {
		time.Sleep(10 * time.Second)
	}
}

func (a *apiProvider) InitiateShutdown() chan struct{} {
	done := make(chan struct{})
	go func() {
		a.s.GracefulStop()
		close(done)
	}()
	return done
}

func (a *apiProvider) RawValues(p *pb.RawValuesParams, r pb.BTrDB_RawValuesServer) error {
	ctx := r.Context()
	uu := p.Uuid
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "read")
	if err != nil {
		return err
	}
	var ep *btrdb.Endpoint
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.readEndpoint(ctx, uu)
		if err != nil {
			continue
		}
		client, err := ep.GetGRPC().RawValues(ctx, p)
		if err != nil {
			continue
		}
		for {
			resp, err := client.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			err = r.Send(resp)
			if err != nil {
				return err
			}
		}
	}
	return err
}
func (a *apiProvider) AlignedWindows(p *pb.AlignedWindowsParams, r pb.BTrDB_AlignedWindowsServer) error {
	ctx := r.Context()
	uu := p.Uuid
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "read")
	if err != nil {
		return err
	}
	var ep *btrdb.Endpoint
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.readEndpoint(ctx, uu)
		if err != nil {
			continue
		}
		client, err := ep.GetGRPC().AlignedWindows(ctx, p)
		if err != nil {
			continue
		}
		for {
			resp, err := client.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			err = r.Send(resp)
			if err != nil {
				return err
			}
		}
	}
	return err
}
func (a *apiProvider) Windows(p *pb.WindowsParams, r pb.BTrDB_WindowsServer) error {
	ctx := r.Context()
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "read")
	if err != nil {
		return err
	}
	var ep *btrdb.Endpoint
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.readEndpoint(ctx, p.Uuid)
		if err != nil {
			continue
		}
		client, err := ep.GetGRPC().Windows(ctx, p)
		if err != nil {
			continue
		}
		for {
			resp, err := client.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			err = r.Send(resp)
			if err != nil {
				return err
			}
		}
	}
	return err
}
func (a *apiProvider) GenerateCSV(p *pb.GenerateCSVParams, r pb.BTrDB_GenerateCSVServer) error {
	ctx := r.Context()
	for _, s := range p.Streams {
		err := a.checkPermissionsByUUID(ctx, s.Uuid, "api", "read")
		if err != nil {
			return err
		}
	}
	var ep *btrdb.Endpoint
	var err error
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.anyEndpoint(ctx)
		if err != nil {
			continue
		}
		client, err := ep.GetGRPC().GenerateCSV(ctx, p)
		if err != nil {
			continue
		}
		for {
			resp, err := client.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			err = r.Send(resp)
			if err != nil {
				return err
			}
		}
	}
	return err
}
func (a *apiProvider) StreamInfo(ctx context.Context, p *pb.StreamInfoParams) (*pb.StreamInfoResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "read")
	if err != nil {
		return nil, err
	}
	var ep *btrdb.Endpoint
	var rv *pb.StreamInfoResponse
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.readEndpoint(ctx, p.Uuid)
		if err != nil {
			continue
		}
		rv, err = ep.GetGRPC().StreamInfo(ctx, p)
	}
	return rv, err
}

func (a *apiProvider) GetMetadataUsage(ctx context.Context, p *pb.MetadataUsageParams) (*pb.MetadataUsageResponse, error) {
	err := a.checkPermissionsByCollection(ctx, p.Prefix, "api", "read")
	if err != nil {
		return nil, err
	}
	var ep *btrdb.Endpoint
	var rv *pb.MetadataUsageResponse
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.anyEndpoint(ctx)
		if err != nil {
			continue
		}
		rv, err = ep.GetGRPC().GetMetadataUsage(ctx, p)
	}
	return rv, err
}

func (a *apiProvider) SetStreamAnnotations(ctx context.Context, p *pb.SetStreamAnnotationsParams) (*pb.SetStreamAnnotationsResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "read")
	if err != nil {
		return nil, err
	}
	var ep *btrdb.Endpoint
	var rv *pb.SetStreamAnnotationsResponse
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.writeEndpoint(ctx, p.Uuid)
		if err != nil {
			continue
		}
		rv, err = ep.GetGRPC().SetStreamAnnotations(ctx, p)
	}
	return rv, err
}
func (a *apiProvider) Changes(p *pb.ChangesParams, r pb.BTrDB_ChangesServer) error {
	ctx := r.Context()
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "read")
	if err != nil {
		return err
	}
	var ep *btrdb.Endpoint
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.readEndpoint(ctx, p.Uuid)
		if err != nil {
			continue
		}
		client, err := ep.GetGRPC().Changes(ctx, p)
		if err != nil {
			continue
		}
		for {
			resp, err := client.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			err = r.Send(resp)
			if err != nil {
				return err
			}
		}
	}
	return err
}
func (a *apiProvider) Create(ctx context.Context, p *pb.CreateParams) (*pb.CreateResponse, error) {
	err := a.checkPermissionsByCollection(ctx, p.Collection, "api", "insert")
	if err != nil {
		return nil, err
	}
	var ep *btrdb.Endpoint
	var rv *pb.CreateResponse
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.writeEndpoint(ctx, p.Uuid)
		if err != nil {
			continue
		}
		rv, err = ep.GetGRPC().Create(ctx, p)
	}
	return rv, err
}
func (a *apiProvider) ListCollections(ctx context.Context, p *pb.ListCollectionsParams) (*pb.ListCollectionsResponse, error) {
	var ep *btrdb.Endpoint
	var rv *pb.ListCollectionsResponse
	var err error
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.anyEndpoint(ctx)
		if err != nil {
			continue
		}
		rv, err = ep.GetGRPC().ListCollections(ctx, p)
	}
	if err != nil {
		return nil, err
	}
	filtCollections := make([]string, 0, len(rv.Collections))
	for _, col := range rv.Collections {
		cerr := a.checkPermissionsByCollection(ctx, col, "api", "read")
		if cerr != nil {
			continue
		}
		filtCollections = append(filtCollections, col)
	}
	rv.Collections = filtCollections
	return rv, err
}
func (a *apiProvider) LookupStreams(p *pb.LookupStreamsParams, r pb.BTrDB_LookupStreamsServer) error {
	ctx := r.Context()
	var ep *btrdb.Endpoint
	var err error
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.anyEndpoint(ctx)
		if err != nil {
			continue
		}
		client, err := ep.GetGRPC().LookupStreams(ctx, p)
		if err != nil {
			continue
		}
		for {
			resp, err := client.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if resp.Stat != nil {
				cerr := r.Send(resp)
				if cerr != nil {
					return cerr
				}
			}
			//Filter the results by permitted ones
			nr := make([]*pb.StreamDescriptor, 0, len(resp.Results))
			for _, res := range resp.Results {
				sterr := a.checkPermissionsByCollection(ctx, res.Collection, "api", "read")
				if sterr != nil {
					continue
				}
				nr = append(nr, res)
			}
			resp.Results = nr
			err = r.Send(resp)
			if err != nil {
				return err
			}
		}
	}
	return err
}
func (a *apiProvider) Nearest(ctx context.Context, p *pb.NearestParams) (*pb.NearestResponse, error) {

	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "read")
	if err != nil {
		return nil, err
	}
	var ep *btrdb.Endpoint
	var rv *pb.NearestResponse
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.readEndpoint(ctx, p.Uuid)
		if err != nil {
			continue
		}
		rv, err = ep.GetGRPC().Nearest(ctx, p)
	}
	return rv, err
}

func (a *apiProvider) Insert(ctx context.Context, p *pb.InsertParams) (*pb.InsertResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "insert")
	if err != nil {
		return nil, err
	}
	var ep *btrdb.Endpoint
	var rv *pb.InsertResponse
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.writeEndpoint(ctx, p.Uuid)
		if err != nil {
			continue
		}
		rv, err = ep.GetGRPC().Insert(ctx, p)
	}
	return rv, err
}
func (a *apiProvider) Delete(ctx context.Context, p *pb.DeleteParams) (*pb.DeleteResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "delete")
	if err != nil {
		return nil, err
	}
	var ep *btrdb.Endpoint
	var rv *pb.DeleteResponse
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.writeEndpoint(ctx, p.GetUuid())
		if err != nil {
			continue
		}
		rv, err = ep.GetGRPC().Delete(ctx, p)
	}
	return rv, err
}

func (a *apiProvider) Flush(ctx context.Context, p *pb.FlushParams) (*pb.FlushResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "insert")
	if err != nil {
		return nil, err
	}
	var ep *btrdb.Endpoint
	var rv *pb.FlushResponse
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.writeEndpoint(ctx, p.GetUuid())
		if err != nil {
			continue
		}
		rv, err = ep.GetGRPC().Flush(ctx, p)
	}
	return rv, err
}

func (a *apiProvider) Obliterate(ctx context.Context, p *pb.ObliterateParams) (*pb.ObliterateResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "api", "obliterate")
	if err != nil {
		return nil, err
	}
	var ep *btrdb.Endpoint
	var rv *pb.ObliterateResponse
	for a.downstream.TestEpError(ep, err) {
		ep, err = a.writeEndpoint(ctx, p.GetUuid())
		if err != nil {
			continue
		}
		rv, err = ep.GetGRPC().Obliterate(ctx, p)
	}
	return rv, err
}

func (a *apiProvider) FaultInject(ctx context.Context, p *pb.FaultInjectParams) (*pb.FaultInjectResponse, error) {
	err := a.checkPermissionsByUUID(ctx, uuid.NewRandom(), "api", "admin")
	if err != nil {
		return nil, err
	}
	ds, err := a.writeEndpoint(ctx, uuid.NewRandom())
	if err != nil {
		return nil, err
	}
	rv, e := ds.GetGRPC().FaultInject(ctx, p)
	return rv, e
}

func (a *apiProvider) Info(ctx context.Context, params *pb.InfoParams) (*pb.InfoResponse, error) {
	//We do not forward the info call, as we want the client to always contact us
	ourip := "localhost"
	if ex := os.Getenv("EXTERNAL_ADDRESS"); ex != "" {
		ourip = ex
	}
	parts := strings.SplitN(ourip, ":", 2)
	ourip = parts[0]
	suffix := ":4410"
	if a.secure {
		suffix = ":4411"
	}
	ProxyInfo := &pb.ProxyInfo{
		ProxyEndpoints: []string{ourip + suffix},
	}
	return &pb.InfoResponse{
		MajorVersion: MajorVersion,
		MinorVersion: MinorVersion,
		Build:        fmt.Sprintf("%d.%d.%d", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch),
		Proxy:        ProxyInfo,
	}, nil
}
