package server

import (
	"net"

	"golang.org/x/net/context"

	cockroach_util "github.com/cockroachdb/cockroach/util"
	"github.com/dmatrixdb/dmatrix/roachpb"
	"github.com/dmatrixdb/dmatrix/rpc"
	"github.com/dmatrixdb/dmatrix/storage"
	"github.com/dmatrixdb/dmatrix/storage/engine"
	"github.com/dmatrixdb/dmatrix/util"
	"github.com/dmatrixdb/dmatrix/util/log"
	"github.com/dmatrixdb/dmatrix/util/stop"
	"github.com/gogo/protobuf/proto"
)

type NodeID int32

type NodeDescriptor struct {
	NodeID  NodeID
	Address cockroach_util.UnresolvedAddr
	Attrs   []string
}

// A Node manages a map of stores (by store ID) for which it serves
// traffic. A node is the top-level data structure. There is one node
// instance per process. A node accepts incoming RPCs and services
// them by directing the commands contained within RPCs to local
// stores, which in turn direct the commands to specific ranges. Each
// node has access to the global, monolithic Key-Value abstraction via
// its kv.DB reference. Nodes use this to allocate node and store
// IDs for bootstrapping the node itself or new stores as they're added
// on subsequent instantiations.
type Node struct {
	ClusterID    string                // UUID for Cockroach cluster
	Descriptor   NodeDescriptor        // Node ID, network/physical topology
	ctx          storage.StoreContext  // Context to use and pass to stores
	storeManager *storage.StoreManager // Access to node-local stores
}

// NewNode returns a new instance of Node.
func NewNode(ctx storage.StoreContext) *Node {
	return &Node{
		ctx: ctx,
	}
}

// context returns a context encapsulating the NodeID.
func (n *Node) context() context.Context {
	return log.Add(context.Background(), log.NodeID, n.Descriptor.NodeID)
}

// initDescriptor initializes the node descriptor with the server
// address and the node attributes.
func (n *Node) initDescriptor(addr net.Addr, attrs roachpb.Attributes) {
	n.Descriptor.Address = util.MakeUnresolvedAddr(addr.Network(), addr.String())
	n.Descriptor.Attrs = attrs
}

func (n *Node) start(rpcServer *rpc.Server, engines []engine.Engine,
	attrs roachpb.Attributes, stopper *stop.Stopper) error {
	n.initDescriptor(rpcServer.Addr(), attrs)
	const method = "Node.Batch"
	if err := rpcServer.Register(method, n.executeCmd, &roachpb.BatchRequest{}); err != nil {
		log.Fatalf("unable to register node service with RPC server: %s", err)
	}

	if err := n.initStoreManager(engines, stopper); err != nil {
		return err
	}

	n.startGossip(stopper)
	log.Infoc(n.context(), "Started node with %v engine(s) and attributes %v", engines, attrs.Attrs)
	return nil
}

func (n *Node) initStoreManager(engines []engine.Engine, stopper *stop.Stopper) error {
	if len(engine) == 0 {
		return util.Errorf("no engine")
	}

	// Just use one store now.
	s := storage.NewStoreManager(n.ctx, engines[0], &n.Descriptor)

	if err := s.Start(stopper); err != nil {
		return util.Errorf("failed to start store: %s", err)
	}

	n.storeManager = s
	return nil
}

// executeCmd interprets the given message as a *roachpb.BatchRequest and sends it
// via the local sender.
func (n *Node) executeCmd(argsI proto.Message) (proto.Message, error) {
	ba := argsI.(*roachpb.BatchRequest)
	br, pErr := n.storeManager.Send(nil, *ba)
	if pErr != nil {
		br = &roachpb.BatchResponse{}
	}
	if br.Error != nil {
		panic(roachpb.ErrorUnexpectedlySet(n.stores, br))
	}
	br.Error = pErr
	return br, nil
}
