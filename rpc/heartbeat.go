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
// Author: Kathy Spradlin (kathyspradlin@gmail.com)
// Author: Spencer Kimball (spencer.kimball@gmail.com)

package rpc

import (
	"fmt"
	"time"

	"github.com/dmatrix/cockroach/security"
	"github.com/dmatrix/cockroach/util/hlc"
	"github.com/dmatrix/cockroach/util/stop"
	"github.com/gogo/protobuf/proto"
)

var _ security.RequestWithUser = &PingRequest{}

// GetUser implements security.RequestWithUser.
// Heartbeat messages are always sent by the node user.
func (*PingRequest) GetUser() string {
	return security.NodeUser
}

// String formats the RemoteOffset for human readability.
func (r RemoteOffset) String() string {
	t := time.Unix(r.MeasuredAt/1E9, 0).UTC()
	return fmt.Sprintf("off=%.9fs, err=%.9fs, at=%s", float64(r.Offset)/1E9, float64(r.Uncertainty)/1E9, t)
}

// A HeartbeatService exposes a method to echo its request params. It doubles
// as a way to measure the offset of the server from other nodes. It uses the
// clock to return the server time every heartbeat. It also keeps track of
// remote clocks sent to it by storing them in the remoteClockMonitor.
type HeartbeatService struct {
	// Provides the nanosecond unix epoch timestamp of the processor.
	clock *hlc.Clock
	// A pointer to the RemoteClockMonitor configured in the RPC Context,
	// shared by rpc clients, to keep track of remote clock measurements.
	remoteClockMonitor *RemoteClockMonitor
}

// Register this service on the given RPC server.
func (hs *HeartbeatService) Register(server *Server) error {
	if err := server.Register("Heartbeat.Ping", hs.Ping, &PingRequest{}); err != nil {
		return err
	}
	return nil
}

// Ping echos the contents of the request to the response, and returns the
// server's current clock value, allowing the requester to measure its clock.
// The requester should also estimate its offset from this server along
// with the requester's address.
func (hs *HeartbeatService) Ping(argsI proto.Message) (proto.Message, error) {
	args := argsI.(*PingRequest)
	reply := &PingResponse{}
	reply.Pong = args.Ping
	serverOffset := args.Offset
	// The server offset should be the opposite of the client offset.
	serverOffset.Offset = -serverOffset.Offset
	hs.remoteClockMonitor.UpdateOffset(args.Addr, serverOffset)
	reply.ServerTime = hs.clock.PhysicalNow()
	return reply, nil
}

// A ManualHeartbeatService allows manual control of when heartbeats occur, to
// facilitate testing.
type ManualHeartbeatService struct {
	clock              *hlc.Clock
	remoteClockMonitor *RemoteClockMonitor
	// Heartbeats are processed when a value is sent here.
	ready   chan struct{}
	stopper *stop.Stopper
}

// Register this service on the given RPC server.
func (mhs *ManualHeartbeatService) Register(server *Server) error {
	if err := server.Register("Heartbeat.Ping", mhs.Ping, &PingRequest{}); err != nil {
		return err
	}
	return nil
}

// Ping waits until the heartbeat service is ready to respond to a Heartbeat.
func (mhs *ManualHeartbeatService) Ping(args proto.Message) (proto.Message, error) {
	select {
	case <-mhs.ready:
	case <-mhs.stopper.ShouldStop():
	}
	hs := HeartbeatService{
		clock:              mhs.clock,
		remoteClockMonitor: mhs.remoteClockMonitor,
	}
	return hs.Ping(args)
}
