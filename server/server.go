package server

import (
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/cockroachdb/cockroach/gossip"
	"github.com/cockroachdb/cockroach/rpc"
	"github.com/cockroachdb/cockroach/util"
	snappy "github.com/dmatrixdb/c-snappy"
	"github.com/dmatrixdb/dmatrix/multiraft"
	"github.com/dmatrixdb/dmatrix/storage"
	"github.com/dmatrixdb/dmatrix/ui"
	"github.com/dmatrixdb/dmatrix/util/hlc"
	"github.com/dmatrixdb/dmatrix/util/log"
	"github.com/dmatrixdb/dmatrix/util/stop"
	assetfs "github.com/elazarl/go-bindata-assetfs"
)

var (
	// Allocation pool for gzip writers.
	gzipWriterPool sync.Pool
	// Allocation pool for snappy writers.
	snappyWriterPool sync.Pool
)

type Server struct {
	ctx           *Context
	gossip        *gossip.Gossip
	mux           *http.ServeMux
	clock         *hlc.Clock
	rpc           *rpc.Server
	node          *Node
	raftTransport multiraft.Transport
	stopper       *stop.Stopper
}

func NewServer(ctx *Context, stopper *stop.Stopper) (*Server, error) {
	if ctx == nil {
		return nil, util.Errorf("ctx must not be null")
	}

	addr := ctx.Addr
	if _, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return nil, util.Errorf("unable to resolve RPC address %q: %v", addr, err)
	}

	s := &Server{
		ctx:     ctx,
		mux:     http.NewServeMux(),
		clock:   hlc.NewClock(hlc.UnixNano),
		stopper: stopper,
	}
	s.clock.SetMaxOffset(ctx.MaxOffset)

	rpcContext := rpc.NewContext(&ctx.Context, s.clock, stopper)
	stopper.RunWorker(func() {
		rpcContext.RemoteClocks.MonitorRemoteOffsets(stopper)
	})

	s.rpc = rpc.NewServer(util.MakeUnresolvedAddr("tcp", addr), rpcContext)
	s.stopper.AddCloser(s.rpc)
	s.gossip = gossip.New(rpcContext, s.ctx.GossipBootstrapResolvers)

	var err error
	s.raftTransport, err = newRPCTransport(s.gossip, s.rpc, rpcContext)
	if err != nil {
		return nil, err
	}
	s.stopper.AddCloser(s.raftTransport)

	nCtx := storage.StoreContext{
		Clock:     s.clock,
		Gossip:    s.gossip,
		Transport: s.raftTransport,
	}
	s.node = NewNode(nCtx)

	return s, nil
}

func (s *Server) Start() error {
	if err := s.rpc.Listen(); err != nil {
		return util.Errorf("could not listen on %s: %s", s.ctx.Addr, err)
	}

	addr := s.rpc.Addr()
	addrStr := addr.String()

	s.gossip.Start(s.rpc, s.stopper)

	if err := s.node.start(s.rpc, s.ctx.Engines, s.ctx.NodeAttributes, s.stopper); err != nil {
		return err
	}

	log.Infof("starting %s server at %s", s.ctx.HTTPRequestScheme(), addr)
	s.initHTTP()
	s.rpc.Serve(s)

	return nil
}

// initHTTP registers http prefixes.
func (s *Server) initHTTP() {
	s.mux.Handle("/", http.FileServer(
		&assetfs.AssetFS{Asset: ui.Asset, AssetDir: ui.AssetDir}))
}

// Stop stops the server.
func (s *Server) Stop() {
	s.stopper.Stop()
}

// ServeHTTP is necessary to implement the http.Handler interface. It
// will snappy a response if the appropriate request headers are set.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if we're draining; if so return 503, service unavailable.
	if !s.stopper.RunTask(func() {
		// Disable caching of responses.
		w.Header().Set("Cache-control", "no-cache")

		ae := r.Header.Get(util.AcceptEncodingHeader)
		switch {
		case strings.Contains(ae, util.SnappyEncoding):
			w.Header().Set(util.ContentEncodingHeader, util.SnappyEncoding)
			s := newSnappyResponseWriter(w)
			defer s.Close()
			w = s
		case strings.Contains(ae, util.GzipEncoding):
			w.Header().Set(util.ContentEncodingHeader, util.GzipEncoding)
			gzw := newGzipResponseWriter(w)
			defer gzw.Close()
			w = gzw
		}
		s.mux.ServeHTTP(w, r)
	}) {
		http.Error(w, "service is draining", http.StatusServiceUnavailable)
	}
}

type gzipResponseWriter struct {
	io.WriteCloser
	http.ResponseWriter
}

func newGzipResponseWriter(w http.ResponseWriter) *gzipResponseWriter {
	var gz *gzip.Writer
	if gzI := gzipWriterPool.Get(); gzI == nil {
		gz = gzip.NewWriter(w)
	} else {
		gz = gzI.(*gzip.Writer)
		gz.Reset(w)
	}
	return &gzipResponseWriter{WriteCloser: gz, ResponseWriter: w}
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.WriteCloser.Write(b)
}

func (w *gzipResponseWriter) Close() {
	if w.WriteCloser != nil {
		w.WriteCloser.Close()
		gzipWriterPool.Put(w.WriteCloser)
		w.WriteCloser = nil
	}
}

type snappyResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func newSnappyResponseWriter(w http.ResponseWriter) *snappyResponseWriter {
	var s *snappy.Writer
	if sI := snappyWriterPool.Get(); sI == nil {
		// TODO(pmattis): It would be better to use the C++ snappy code
		// like rpc/codec is doing. Would have to copy the snappy.Writer
		// implementation from snappy-go.
		s = snappy.NewWriter(w)
	} else {
		s = sI.(*snappy.Writer)
		s.Reset(w)
	}
	return &snappyResponseWriter{Writer: s, ResponseWriter: w}
}

func (w *snappyResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *snappyResponseWriter) Close() {
	if w.Writer != nil {
		snappyWriterPool.Put(w.Writer)
		w.Writer = nil
	}
}
