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

	"os"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/clientv3"

	btrdb "gopkg.in/BTrDB/btrdb.v4"

	"github.com/BTrDB/smartgridstore/tools"
	"github.com/BTrDB/smartgridstore/tools/apifrontend/cli"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	logging "github.com/op/go-logging"
	"github.com/pborman/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	pb "gopkg.in/BTrDB/btrdb.v4/grpcinterface"
)

const MajorVersion = tools.VersionMajor
const MinorVersion = tools.VersionMinor

type apiProvider struct {
	s          *grpc.Server
	downstream *btrdb.BTrDB
}

var logger *logging.Logger

func init() {
	logger = logging.MustGetLogger("log")
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
func (a *apiProvider) checkPermissionsByUUID(ctx context.Context, uu uuid.UUID, op string) error {
	return nil
}
func (a *apiProvider) checkPermissionsByCollection(ctx context.Context, collection string, op string) error {
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

	var cfg *tls.Config

	mode, err := cli.GetAPIFrontendCertSrc(etcdClient)

	if err != nil {
		fmt.Printf("could not get certificate config: %v\n", err)
		os.Exit(1)
	}
	switch mode {
	case "autocert":
		cfg, err = MRPlottersAutocertTLSConfig(etcdClient)
		if err != nil {
			fmt.Printf("could not set up autocert: %v\n", err)
			os.Exit(1)
		}
		if cfg == nil {
			fmt.Printf("mrplotter autocert not set up fully\n")
			os.Exit(1)
		}
		fmt.Printf("successfully loaded mrplotter's cert\n")
	case "hardcoded":
		cert, key, err := cli.GetAPIFrontendHardcoded(etcdClient)
		if err != nil {
			fmt.Printf("could not load hardcoded certificate: %v\n", err)
			os.Exit(1)
		}
		if len(cert) == 0 || len(key) == 0 {
			fmt.Printf("CRITICAL: certsrc set to hardcoded but no certificate set\n")
			os.Exit(1)
		}
		var tlsCertificate tls.Certificate
		tlsCertificate, err = tls.X509KeyPair(cert, key)
		cfg = &tls.Config{
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				return &tlsCertificate, nil
			},
		}
	default:
		fmt.Printf("WARNING! secure API disabled in api frontend\n")
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
	grpcServer := grpc.NewServer(grpc.Creds(creds))
	api := &apiProvider{s: grpcServer,
		downstream: downstream,
	}
	pb.RegisterBTrDBServer(grpcServer, api)
	go grpcServer.Serve(l)
	return cfg
}

func ProxyGRPC(laddr string) GRPCInterface {
	// go func() {
	// 	fmt.Println("==== PROFILING ENABLED ==========")
	// 	err := http.ListenAndServe("0.0.0.0:6061", nil)
	// 	panic(err)
	// }()
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
	grpcServer := grpc.NewServer()
	api := &apiProvider{s: grpcServer,
		downstream: downstream,
	}
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
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "RawValues")
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
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "AlignedWindows")
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
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Windows")
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
func (a *apiProvider) StreamInfo(ctx context.Context, p *pb.StreamInfoParams) (*pb.StreamInfoResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "StreamInfo")
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
	err := a.checkPermissionsByCollection(ctx, p.Prefix, "GetMetadataUsage")
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
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "SetStreamAnnotations")
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
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Changes")
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
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Create")
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
		cerr := a.checkPermissionsByCollection(ctx, col, "ListCollections")
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
				sterr := a.checkPermissionsByCollection(ctx, res.Collection, "LookupStreams")
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

	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Nearest")
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
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Insert")
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
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Delete")
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
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Flush")
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
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Obliterate")
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
	err := a.checkPermissionsByUUID(ctx, uuid.NewRandom(), "FaultInject")
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
	ourip := "127.0.0.1:4410"
	if ex := os.Getenv("EXTERNAL_ADDRESS"); ex != "" {
		ourip = ex
	}
	ProxyInfo := &pb.ProxyInfo{
		ProxyEndpoints: []string{ourip},
	}
	return &pb.InfoResponse{
		MajorVersion: MajorVersion,
		MinorVersion: MinorVersion,
		Build:        fmt.Sprintf("%d.%d.%d", tools.VersionMajor, tools.VersionMinor, tools.VersionPatch),
		Proxy:        ProxyInfo,
	}, nil
}
