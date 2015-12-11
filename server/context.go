// Copyright 2015 The Cockroach Authors.
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
// Author: Daniel Theophanes (kardianos@gmail.com)

package server

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/gossip/resolver"
	"github.com/cockroachdb/cockroach/util"
	"github.com/cockroachdb/cockroach/util/log"
	"github.com/cockroachdb/cockroach/util/stop"
	"github.com/dmatrixdb/dmatrix/base"
	"github.com/dmatrixdb/dmatrix/roachpb"
	"github.com/dmatrixdb/dmatrix/storage/engine"
)

// Context defaults.
const (
	defaultAddr           = ":26257"
	defaultMaxOffset      = 250 * time.Millisecond
	defaultCacheSize      = 512 << 20 // 512 MB
	defaultMemtableBudget = 512 << 20 // 512 MB
)

// Context holds parameters needed to setup a server.
// Calling "cli".initFlags(ctx *Context) will initialize Context using
// command flags. Keep in sync with "cli/flags.go".
type Context struct {
	// Embed the base context.
	base.Context

	// Addr is the host:port to bind for HTTP/RPC traffic.
	Addr string

	// Stores is specified to enable durable key-value storage.
	// Memory-backed key value stores may be optionally specified
	// via mem=<integer byte size>.
	//
	// Stores specify a comma-separated list of stores specified by a
	// colon-separated list of device attributes followed by '=' and
	// either a filepath for a persistent store or an integer size in bytes for an
	// in-memory store. Device attributes typically include whether the store is
	// flash (ssd), spinny disk (hdd), fusion-io (fio), in-memory (mem); device
	// attributes might also include speeds and other specs (7200rpm, 200kiops, etc.).
	// For example, -store=hdd:7200rpm=/mnt/hda1,ssd=/mnt/ssd01,ssd=/mnt/ssd02,mem=1073741824
	Stores string

	// Attrs specifies a colon-separated list of node topography or machine
	// capabilities, used to match capabilities or location preferences specified
	// in zone configs.
	Attrs string

	// Maximum clock offset for the cluster.
	MaxOffset time.Duration

	// GossipBootstrap is a comma-separated list of node addresses that
	// act as bootstrap hosts for connecting to the gossip network.
	GossipBootstrap string

	// GossipBootstrapResolvers is a list of gossip resolvers used
	// to find bootstrap nodes for connecting to the gossip network.
	GossipBootstrapResolvers []resolver.Resolver

	// CacheSize is the amount of memory in bytes to use for caching data.
	// The value is split evenly between the stores if there are more than one.
	CacheSize int64

	// MemtableBudget is the amount of memory in bytes to use for the memory
	// table. The value is split evenly between the stores if there are more than one.
	MemtableBudget int64

	// Engines is the storage instances specified by Stores.
	Engines []engine.Engine

	// NodeAttributes is the parsed representation of Attrs.
	NodeAttributes roachpb.Attributes
}

// NewContext returns a Context with default values.
func NewContext() *Context {
	ctx := &Context{}
	ctx.InitDefaults()
	return ctx
}

// InitDefaults sets up the default values for a Context.
func (ctx *Context) InitDefaults() {
	ctx.Context.InitDefaults()
	ctx.Addr = defaultAddr
	ctx.MaxOffset = defaultMaxOffset
	ctx.CacheSize = defaultCacheSize
	ctx.MemtableBudget = defaultMemtableBudget
}

// Get the stores on both start and init.
var storesRE = regexp.MustCompile(`([^=]+)=([^,]+)(,|$)`)

// InitStores interprets the stores parameter to initialize a slice of
// engine.Engine objects.
func (ctx *Context) InitStores(stopper *stop.Stopper) error {
	storeSpecs := storesRE.FindAllStringSubmatch(ctx.Stores, -1)
	// Error if regexp doesn't match.
	if storeSpecs == nil {
		return fmt.Errorf("invalid or empty engines specification %q, did you specify --stores?", ctx.Stores)
	}

	for _, storeSpec := range storeSpecs {
		name := storeSpec[0]
		if len(storeSpec) != 4 {
			return util.Errorf("unable to parse attributes and path from store %q", name)
		}
		attrs, path := storeSpec[1], storeSpec[2]
		// There are two matches for each store specification: the colon-separated
		// list of attributes and the path.
		engine, err := ctx.initEngine(attrs, path, stopper)
		if err != nil {
			return util.Errorf("unable to init engine for store %q: %s", name, err)
		}
		ctx.Engines = append(ctx.Engines, engine)
	}
	log.Infof("initialized %d storage engine(s)", len(ctx.Engines))
	return nil
}

func (ctx *Context) InitNode() error {
	// Initialize attributes.
	ctx.NodeAttributes = parseAttributes(ctx.Attrs)
	// Get the gossip bootstrap resolvers.
	resolvers, err := ctx.parseGossipBootstrapResolvers()
	if err != nil {
		return err
	}
	if len(resolvers) == 0 {
		return errNoGossipAddresses
	}
	ctx.GossipBootstrapResolvers = resolvers

	return nil
}

func (ctx *Context) initEngine(attrsStr, path string, stopper *stop.Stopper) (engine.Engine, error) {
	attrs := parseAttributes(attrsStr)
	return engine.NewRocksDB(attrs, path, ctx.CacheSize, ctx.MemtableBudget, stopper), nil
}

// SelfGossipAddr is a special flag that configures a node to gossip
// only with itself. This avoids having to specify the port twice for
// single-node clusters (i.e. once in --addr, and again in --gossip).
const SelfGossipAddr = "self="

// parseGossipBootstrapResolvers parses a comma-separated list of
// gossip bootstrap resolvers.
func (ctx *Context) parseGossipBootstrapResolvers() ([]resolver.Resolver, error) {
	var bootstrapResolvers []resolver.Resolver
	addresses := strings.Split(ctx.GossipBootstrap, ",")
	for _, address := range addresses {
		if len(address) == 0 {
			continue
		}
		if strings.HasPrefix(address, SelfGossipAddr) {
			address = ctx.Addr
		}
		resolver, err := resolver.NewResolver(&ctx.Context, address)
		if err != nil {
			return nil, err
		}
		bootstrapResolvers = append(bootstrapResolvers, resolver)
	}

	return bootstrapResolvers, nil
}

// parseAttributes parses a colon-separated list of strings,
// filtering empty strings (i.e. "::" will yield no attributes.
// Returns the list of strings as Attributes.
func parseAttributes(attrsStr string) roachpb.Attributes {
	var filtered []string
	for _, attr := range strings.Split(attrsStr, ":") {
		if len(attr) != 0 {
			filtered = append(filtered, attr)
		}
	}
	return roachpb.Attributes{Attrs: filtered}
}
