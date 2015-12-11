// Copyright 2014 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.
//
// Author: Spencer Kimball (spencer.kimball@gmail.com)

package storage

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/cockroachdb/cockroach/multiraft"
	"github.com/cockroachdb/cockroach/util"
	"github.com/cockroachdb/cockroach/util/hlc"
	"github.com/cockroachdb/cockroach/util/log"
	"github.com/cockroachdb/cockroach/util/stop"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/dmatrixdb/dmatrix/gossip"
	"github.com/dmatrixdb/dmatrix/keys"
	"github.com/dmatrixdb/dmatrix/roachpb"
	"github.com/dmatrixdb/dmatrix/server"
	"github.com/dmatrixdb/dmatrix/storage/engine"
)

const (
	defaultRaftTickInterval         = 100 * time.Millisecond
	defaultHeartbeatIntervalTicks   = 3
	defaultRaftElectionTimeoutTicks = 15
	// ttlStoreGossip is time-to-live for store-related info.
	ttlStoreGossip = 2 * time.Minute
)

var changeTypeInternalToRaft = map[roachpb.ReplicaChangeType]raftpb.ConfChangeType{
	roachpb.ADD_REPLICA:    raftpb.ConfChangeAddNode,
	roachpb.REMOVE_REPLICA: raftpb.ConfChangeRemoveNode,
}

type StoreID int32
type GroupID int32

type StoreIdent struct {
	NodeID  server.NodeID
	StoreID StoreID
}

type ReplicaDescriptor struct {
	NodeID  server.NodeID
	StoreID StoreID
}

type GroupDescriptor struct {
	GroupID  GroupID
	Replicas []ReplicaDescriptor
}

type StoreManager struct {
	Ident  StoreIdent
	ctx    StoreContext
	engine engine.Engine        // The underlying key-value store
	groups map[GroupID]*Replica // Map of replicas by Group ID

	proposeChan chan proposeOp
	multiraft   *multiraft.MultiRaft
	stopper     *stop.Stopper
	nodeDesc    *roachpb.NodeDescriptor
	mu          sync.RWMutex // Protects variables below...
}

var _ multiraft.Storage = &StoreManager{}

type StoreContext struct {
	Clock     *hlc.Clock
	Gossip    *gossip.Gossip
	Transport multiraft.Transport

	// RaftTickInterval is the resolution of the Raft timer; other raft timeouts
	// are defined in terms of multiples of this value.
	RaftTickInterval time.Duration

	// RaftHeartbeatIntervalTicks is the number of ticks that pass between heartbeats.
	RaftHeartbeatIntervalTicks int

	// RaftElectionTimeoutTicks is the number of ticks that must pass before a follower
	// considers a leader to have failed and calls a new election. Should be significantly
	// higher than RaftHeartbeatIntervalTicks. The raft paper recommends a value of 150ms
	// for local networks.
	RaftElectionTimeoutTicks int
}

// Valid returns true if the StoreContext is populated correctly.
// We don't check for Gossip and DB since some of our tests pass
// that as nil.
func (sc *StoreContext) Valid() bool {
	return sc.Clock != nil && sc.Transport != nil &&
		sc.RaftTickInterval != 0 && sc.RaftHeartbeatIntervalTicks > 0
}

func (sc *StoreContext) setDefaults() {
	if sc.RaftTickInterval == 0 {
		sc.RaftTickInterval = defaultRaftTickInterval
	}
	if sc.RaftHeartbeatIntervalTicks == 0 {
		sc.RaftHeartbeatIntervalTicks = defaultHeartbeatIntervalTicks
	}
	if sc.RaftElectionTimeoutTicks == 0 {
		sc.RaftElectionTimeoutTicks = defaultRaftElectionTimeoutTicks
	}
}

func NewStoreManager(ctx StoreContext, eng engine.Engine, nodeDesc *server.NodeDescriptor) *StoreManager {
	ctx.setDefaults()

	if !ctx.Valid() {
		panic(fmt.Sprintf("invalid store configuration: %+v", &ctx))
	}

	s := &StoreManager{
		Ident: StoreIdent{
			NodeID:  nodeDesc.NodeID,
			StoreID: 1,
		},
		ctx:         ctx,
		engine:      eng,
		groups:      map[GroupID]*Replica{},
		nodeDesc:    nodeDesc,
		proposeChan: make(chan proposeOp),
	}

	return s
}

// String formats a store for debug output.
func (s *StoreManager) String() string {
	return fmt.Sprintf("store=%d:%d (%s)", s.Ident.NodeID, s.Ident.StoreID, s.engine)
}

// Context returns a base context to pass along with commands being executed,
// derived from the supplied context (which is allowed to be nil).
func (s *StoreManager) Context(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return log.Add(ctx,
		log.NodeID, s.Ident.NodeID,
		log.StoreID, s.Ident.StoreID)
}

func (s *StoreManager) Start(stopper *stop.Stopper) error {
	s.stopper = stopper

	if err := s.engine.Open(); err != nil {
		return err
	}

	if s.multiraft, err = multiraft.NewMultiRaft(s.Ident.NodeID, s.Ident.StoreID, &multiraft.Config{
		Transport:              s.ctx.Transport,
		Storage:                s,
		TickInterval:           s.ctx.RaftTickInterval,
		ElectionTimeoutTicks:   s.ctx.RaftElectionTimeoutTicks,
		HeartbeatIntervalTicks: s.ctx.RaftHeartbeatIntervalTicks,
		EntryFormatter:         raftEntryFormatter,
	}, s.stopper); err != nil {
		return err
	}

	s.multiraft.Start()
	s.processRaft()

	return nil
}

// RaftStatus returns the current raft status of the given range.
func (s *StoreManager) RaftStatus(rangeID roachpb.RangeID) *raft.Status {
	return s.multiraft.Status(rangeID)
}

// The following methods implement the RangeManager interface.

// StoreID accessor.
func (s *StoreManager) StoreID() roachpb.StoreID { return s.Ident.StoreID }

// Clock accessor.
func (s *StoreManager) Clock() *hlc.Clock { return s.ctx.Clock }

// Engine accessor.
func (s *StoreManager) Engine() engine.Engine { return s.engine }

// Stopper accessor.
func (s *StoreManager) Stopper() *stop.Stopper { return s.stopper }

// NewSnapshot creates a new snapshot engine.
func (s *StoreManager) NewSnapshot() engine.Engine {
	return s.engine.NewSnapshot()
}

// Send fetches a range based on the header's replica, assembles
// method, args & reply into a Raft Cmd struct and executes the
// command using the fetched range.
func (s *StoreManager) Send(ctx context.Context, ba roachpb.BatchRequest) (*roachpb.BatchResponse, *roachpb.Error) {
	ctx = s.Context(ctx)

	var br *roachpb.BatchResponse
	{
		var pErr *roachpb.Error
		br, pErr = s.groups[1].Send(ctx, ba)
		err = pErr.GoError()
	}

	if err == nil {
		return br, nil
	}
}

type proposeOp struct {
	idKey cmdIDKey
	cmd   roachpb.RaftCommand
	ch    chan<- <-chan error
}

// ProposeRaftCommand submits a command to raft. The command is processed
// asynchronously and an error or nil will be written to the returned
// channel when it is committed or aborted (but note that committed does
// mean that it has been applied to the range yet).
func (s *StoreManager) ProposeRaftCommand(idKey cmdIDKey, cmd roachpb.RaftCommand) <-chan error {
	ch := make(chan (<-chan error))
	s.proposeChan <- proposeOp{idKey, cmd, ch}
	return <-ch
}

// proposeRaftCommandImpl runs on the processRaft goroutine.
func (s *StoreManager) proposeRaftCommandImpl(idKey cmdIDKey, cmd roachpb.RaftCommand) <-chan error {
	// If the range has been removed since the proposal started, drop it now.
	s.mu.RLock()
	_, ok := s.replicas[cmd.RangeID]
	s.mu.RUnlock()
	if !ok {
		ch := make(chan error, 1)
		ch <- roachpb.NewRangeNotFoundError(cmd.RangeID)
		return ch
	}
	// Lazily create group.
	if err := s.multiraft.CreateGroup(cmd.RangeID); err != nil {
		ch := make(chan error, 1)
		ch <- err
		return ch
	}

	data, err := proto.Marshal(&cmd)
	if err != nil {
		log.Fatal(err)
	}
	for _, union := range cmd.Cmd.Requests {
		args := union.GetInner()
		etr, ok := args.(*roachpb.EndTransactionRequest)
		if ok {
			if crt := etr.InternalCommitTrigger.GetChangeReplicasTrigger(); crt != nil {
				// TODO(tschottdorf): the real check is that EndTransaction needs
				// to be the last element in the batch. Any caveats to solve before
				// changing this?
				if len(cmd.Cmd.Requests) != 1 {
					panic("EndTransaction should only ever occur by itself in a batch")
				}
				// EndTransactionRequest with a ChangeReplicasTrigger is special because raft
				// needs to understand it; it cannot simply be an opaque command.
				log.Infof("raft: %s %v for range %d", crt.ChangeType, crt.Replica, cmd.RangeID)
				return s.multiraft.ChangeGroupMembership(cmd.RangeID, string(idKey),
					changeTypeInternalToRaft[crt.ChangeType],
					crt.Replica,
					data)
			}
		}
	}
	return s.multiraft.SubmitCommand(cmd.RangeID, string(idKey), data)
}

// processRaft processes write commands that have been committed
// by the raft consensus algorithm, dispatching them to the
// appropriate range. This method starts a goroutine to process Raft
// commands indefinitely or until the stopper signals.
func (s *StoreManager) processRaft() {
	s.stopper.RunWorker(func() {
		for {
			select {
			case events := <-s.multiraft.Events:
				for _, e := range events {
					var cmd roachpb.RaftCommand
					var groupID roachpb.RangeID
					var commandID string
					var index uint64
					var callback func(error)

					switch e := e.(type) {
					case *multiraft.EventCommandCommitted:
						groupID = e.GroupID
						commandID = e.CommandID
						index = e.Index
						err := proto.Unmarshal(e.Command, &cmd)
						if err != nil {
							log.Fatal(err)
						}
						if log.V(6) {
							log.Infof("store %s: new committed command at index %d", s, e.Index)
						}

					case *multiraft.EventMembershipChangeCommitted:
						groupID = e.GroupID
						commandID = e.CommandID
						index = e.Index
						callback = e.Callback
						err := proto.Unmarshal(e.Payload, &cmd)
						if err != nil {
							log.Fatal(err)
						}
						if log.V(6) {
							log.Infof("store %s: new committed membership change at index %d", s, e.Index)
						}

					default:
						continue
					}

					if groupID != cmd.RangeID {
						log.Fatalf("e.GroupID (%d) should == cmd.RangeID (%d)", groupID, cmd.RangeID)
					}

					s.mu.RLock()
					r, ok := s.replicas[groupID]
					s.mu.RUnlock()
					var err error
					if !ok {
						err = util.Errorf("got committed raft command for %d but have no range with that ID: %+v",
							groupID, cmd)
						log.Error(err)
					} else {
						err = r.processRaftCommand(cmdIDKey(commandID), index, cmd)
					}
					if callback != nil {
						callback(err)
					}
				}

			case op := <-s.removeReplicaChan:
				op.ch <- s.removeReplicaImpl(op.rep, op.origDesc)

			case op := <-s.proposeChan:
				op.ch <- s.proposeRaftCommandImpl(op.idKey, op.cmd)

			case <-s.stopper.ShouldStop():
				return
			}
		}
	})
}

// GroupStorage implements the multiraft.Storage interface.
// The caller must hold the store's lock.
func (s *StoreManager) GroupStorage(groupID roachpb.RangeID, replicaID roachpb.ReplicaID) (multiraft.WriteableGroupStorage, error) {
	r, ok := s.groups[groupID]
	if !ok {
		// Before creating the group, see if there is a tombstone which
		// would indicate that this is a stale message.
		tombstoneKey := keys.RaftTombstoneKey(groupID)
		var tombstone roachpb.RaftTombstone
		if ok, err := engine.MVCCGetProto(s.Engine(), tombstoneKey, roachpb.ZeroTimestamp, true, nil, &tombstone); err != nil {
			return nil, err
		} else if ok {
			if replicaID != 0 && replicaID < tombstone.NextReplicaID {
				return nil, multiraft.ErrGroupDeleted
			}
		}

		var err error
		r, err = NewReplica(&roachpb.RangeDescriptor{
			RangeID: groupID,
			// TODO(bdarnell): other fields are unknown; need to populate them from
			// snapshot.
		}, s)
		if err != nil {
			return nil, err
		}
		// Add the range to range map, but not rangesByKey since
		// the range's start key is unknown. The range will be
		// added to rangesByKey later when a snapshot is applied.
		if err = s.addReplicaToRangeMap(r); err != nil {
			return nil, err
		}
		s.uninitReplicas[r.Desc().RangeID] = r
	}
	return r, nil
}

// ReplicaDescriptor implements the multiraft.Storage interface.
// The caller must hold the store's lock.
func (s *StoreManager) ReplicaDescriptor(groupID roachpb.RangeID, replicaID roachpb.ReplicaID) (roachpb.ReplicaDescriptor, error) {
	rep, err := s.getReplicaLocked(groupID)
	if err != nil {
		return roachpb.ReplicaDescriptor{}, err
	}
	return rep.ReplicaDescriptor(replicaID)
}

// ReplicaIDForStore implements the multiraft.Storage interface.
// The caller must hold the store's lock.
func (s *StoreManager) ReplicaIDForStore(groupID roachpb.RangeID, storeID roachpb.StoreID) (roachpb.ReplicaID, error) {
	r, err := s.getReplicaLocked(groupID)
	if err != nil {
		return 0, err
	}
	for _, rep := range r.Desc().Replicas {
		if rep.StoreID == storeID {
			return rep.ReplicaID, nil
		}
	}
	return 0, util.Errorf("store %s not found as replica of range %d", storeID, groupID)
}

// RaftLocker implements the multiraft.Storage interface.
func (s *StoreManager) RaftLocker() sync.Locker {
	return &s.mu
}

// CanApplySnapshot implements the multiraft.Storage interface.
// The caller must hold the store's lock.
func (s *StoreManager) CanApplySnapshot(rangeID roachpb.RangeID, snap raftpb.Snapshot) bool {
	if r, ok := s.replicas[rangeID]; ok && r.isInitialized() {
		// We have the range and it's initialized, so let the snapshot
		// through.
		return true
	}

	// We don't have the range (or we have an uninitialized
	// placeholder). Will we be able to create/initialize it?
	// TODO(bdarnell): can we avoid parsing this twice?
	var parsedSnap roachpb.RaftSnapshotData
	if err := parsedSnap.Unmarshal(snap.Data); err != nil {
		return false
	}

	if s.hasOverlappingReplicaLocked(&parsedSnap.RangeDescriptor) {
		// We have a conflicting range, so we must block the snapshot.
		// When such a conflict exists, it will be resolved by one range
		// either being split or garbage collected.
		return false
	}

	return true
}

// AppliedIndex implements the multiraft.Storage interface.
// The caller must hold the store's lock.
func (s *StoreManager) AppliedIndex(groupID roachpb.RangeID) (uint64, error) {
	r, err := s.getReplicaLocked(groupID)
	if err != nil {
		return 0, err
	}
	return atomic.LoadUint64(&r.appliedIndex), nil
}

func raftEntryFormatter(data []byte) string {
	if len(data) == 0 {
		return "[empty]"
	}
	var cmd roachpb.RaftCommand
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Sprintf("[error parsing entry: %s]", err)
	}
	s := cmd.String()
	maxLen := 300
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}
