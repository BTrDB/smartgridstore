package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	btrdb "gopkg.in/BTrDB/btrdb.v4"

	"github.com/BTrDB/smartgridstore/tools"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	logging "github.com/op/go-logging"
	"github.com/pborman/uuid"
	"google.golang.org/grpc"
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

// func (a *apiProvider) getUser(ctx context.Context) (*User, error) {
// 	return nil, nil
// }
func (a *apiProvider) checkPermissionsByUUID(ctx context.Context, uu uuid.UUID, op string) error {
	return nil
}
func (a *apiProvider) checkPermissionsByCollection(ctx context.Context, collection string, op string) error {
	return nil
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

	ProxyGRPC("0.0.0.0:4410")

	// etcdEndpoint := os.Getenv("ETCD_ENDPOINT")
	// if len(etcdEndpoint) == 0 {
	// 	etcdEndpoint = "http://etcd:2379"
	// }
	// etcdClient, err = etcd.New(etcd.Config{
	// 	Endpoints:   []string{etcdEndpoint},
	// 	DialTimeout: 5 * time.Second})
	// if err != nil {
	// 	fmt.Printf("Could not connect to etcd: %v\n", err)
	// 	os.Exit(1)
	// }

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
	err = http.ListenAndServe(":9000", mux)
	panic(err)

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
	ds, err := a.readEndpoint(ctx, uu)
	if err != nil {
		return err
	}
	client, err := ds.GetGRPC().RawValues(ctx, p)
	if err != nil {
		return err
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
func (a *apiProvider) AlignedWindows(p *pb.AlignedWindowsParams, r pb.BTrDB_AlignedWindowsServer) error {
	ctx := r.Context()
	uu := p.Uuid
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "AlignedWindows")
	if err != nil {
		return err
	}
	ds, err := a.readEndpoint(ctx, uu)
	if err != nil {
		return err
	}
	client, err := ds.GetGRPC().AlignedWindows(ctx, p)
	if err != nil {
		return err
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
func (a *apiProvider) Windows(p *pb.WindowsParams, r pb.BTrDB_WindowsServer) error {
	ctx := r.Context()
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Windows")
	if err != nil {
		return err
	}
	ds, err := a.readEndpoint(ctx, p.GetUuid())
	if err != nil {
		return err
	}
	client, err := ds.GetGRPC().Windows(ctx, p)
	if err != nil {
		return err
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
func (a *apiProvider) StreamInfo(ctx context.Context, p *pb.StreamInfoParams) (*pb.StreamInfoResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "StreamInfo")
	if err != nil {
		return nil, err
	}
	ds, err := a.readEndpoint(ctx, p.GetUuid())
	if err != nil {
		return nil, err
	}
	return ds.GetGRPC().StreamInfo(ctx, p)
}

func (a *apiProvider) SetStreamAnnotations(ctx context.Context, p *pb.SetStreamAnnotationsParams) (*pb.SetStreamAnnotationsResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "SetStreamAnnotations")
	if err != nil {
		return nil, err
	}
	ds, err := a.writeEndpoint(ctx, p.GetUuid())
	if err != nil {
		return nil, err
	}
	return ds.GetGRPC().SetStreamAnnotations(ctx, p)
}
func (a *apiProvider) Changes(p *pb.ChangesParams, r pb.BTrDB_ChangesServer) error {
	ctx := r.Context()
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Changes")
	if err != nil {
		return err
	}
	ds, err := a.readEndpoint(ctx, p.GetUuid())
	if err != nil {
		return err
	}
	client, err := ds.GetGRPC().Changes(ctx, p)
	if err != nil {
		return err
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
func (a *apiProvider) Create(ctx context.Context, p *pb.CreateParams) (*pb.CreateResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Create")
	if err != nil {
		return nil, err
	}
	ds, err := a.writeEndpoint(ctx, p.GetUuid())
	if err != nil {
		return nil, err
	}
	return ds.GetGRPC().Create(ctx, p)
}
func (a *apiProvider) ListCollections(ctx context.Context, p *pb.ListCollectionsParams) (*pb.ListCollectionsResponse, error) {
	// fmt.Printf("Got LC\n")
	// fmt.Printf("context on LC was: %v\n", ctx)
	// md, ok := metadata.FromIncomingContext(ctx)
	// fmt.Printf("ok was: %v\n", ok)
	// fmt.Printf("explicit auth value: %v\n", md["authorization"])
	ds, err := a.readEndpoint(ctx, uuid.NewRandom())
	if err != nil {
		return nil, err
	}
	lcr, err := ds.GetGRPC().ListCollections(ctx, p)
	if err != nil {
		return nil, err
	}
	filtCollections := make([]string, 0, len(lcr.Collections))
	for _, col := range lcr.Collections {
		cerr := a.checkPermissionsByCollection(ctx, col, "ListCollections")
		if cerr != nil {
			continue
		}
		filtCollections = append(filtCollections, col)
	}
	lcr.Collections = filtCollections
	return lcr, err
}
func (a *apiProvider) LookupStreams(p *pb.LookupStreamsParams, r pb.BTrDB_LookupStreamsServer) error {
	ctx := r.Context()
	ds, err := a.readEndpoint(ctx, uuid.NewRandom())
	if err != nil {
		return err
	}
	client, err := ds.GetGRPC().LookupStreams(ctx, p)
	if err != nil {
		return err
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
func (a *apiProvider) Nearest(ctx context.Context, p *pb.NearestParams) (*pb.NearestResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Nearest")
	if err != nil {
		return nil, err
	}
	ds, err := a.readEndpoint(ctx, p.GetUuid())
	if err != nil {
		return nil, err
	}
	return ds.GetGRPC().Nearest(ctx, p)
}

func (a *apiProvider) Insert(ctx context.Context, p *pb.InsertParams) (*pb.InsertResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Insert")
	if err != nil {
		return nil, err
	}
	ds, err := a.writeEndpoint(ctx, p.GetUuid())
	if err != nil {
		return nil, err
	}
	rv, e := ds.GetGRPC().Insert(ctx, p)
	return rv, e
}
func (a *apiProvider) Delete(ctx context.Context, p *pb.DeleteParams) (*pb.DeleteResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Delete")
	if err != nil {
		return nil, err
	}
	ds, err := a.writeEndpoint(ctx, p.GetUuid())
	if err != nil {
		return nil, err
	}
	return ds.GetGRPC().Delete(ctx, p)
}

func (a *apiProvider) Flush(ctx context.Context, p *pb.FlushParams) (*pb.FlushResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Flush")
	if err != nil {
		return nil, err
	}
	ds, err := a.writeEndpoint(ctx, p.GetUuid())
	if err != nil {
		return nil, err
	}
	rv, e := ds.GetGRPC().Flush(ctx, p)
	return rv, e
}

func (a *apiProvider) Obliterate(ctx context.Context, p *pb.ObliterateParams) (*pb.ObliterateResponse, error) {
	err := a.checkPermissionsByUUID(ctx, p.GetUuid(), "Obliterate")
	if err != nil {
		return nil, err
	}
	ds, err := a.writeEndpoint(ctx, p.GetUuid())
	if err != nil {
		return nil, err
	}
	rv, e := ds.GetGRPC().Obliterate(ctx, p)
	return rv, e
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
	// nevertheless tihs PARTICULAR repsonse is a hack
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
