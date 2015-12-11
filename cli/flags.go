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

package cli

import (
	"flag"
	"reflect"
	"strings"

	"github.com/dmatrixdb/dmatrix/server"
	"github.com/spf13/cobra"
)

var maxResults int64

// pflagValue wraps flag.Value and implements the extra methods of the
// pflag.Value interface.
type pflagValue struct {
	flag.Value
}

func (v pflagValue) Type() string {
	t := reflect.TypeOf(v.Value).Elem()
	return t.Kind().String()
}

func (v pflagValue) IsBoolFlag() bool {
	t := reflect.TypeOf(v.Value).Elem()
	return t.Kind() == reflect.Bool
}

var flagUsage = map[string]string{
	"addr": `
        The host:port to bind for HTTP/RPC traffic.
`,
	"attrs": `
        An ordered, colon-separated list of node attributes. Attributes are
        arbitrary strings specifying topography or machine
        capabilities. Topography might include datacenter designation
        (e.g. "us-west-1a", "us-west-1b", "us-east-1c"). Machine capabilities
        might include specialized hardware or number of cores (e.g. "gpu",
        "x16c"). The relative geographic proximity of two nodes is inferred
        from the common prefix of the attributes list, so topographic
        attributes should be specified first and in the same order for all
        nodes. For example:

          --attrs=us-west-1b,gpu.
`,
	"cache-size": `
        Total size in bytes for caches, shared evenly if there are multiple
        storage devices.
`,
	"gossip": `
        A comma-separated list of gossip addresses or resolvers for gossip
        bootstrap. Each item in the list has an optional type:
        [type=]<address>. An unspecified type means ip address or dns.
        Type is one of:
        - tcp: (default if type is omitted): plain ip address or hostname,
          or "self" for single-node systems.
        - unix: unix socket
        - lb: RPC load balancer forwarding to an arbitrary node
        - http-lb: HTTP load balancer: we query
          http(s)://<address>/_status/details/local
`,
	"key-size": `
        Key size in bits for CA/Node/Client certificates.
`,
	"max-offset": `
        The maximum clock offset for the cluster. Clock offset is measured on
        all node-to-node links and if any node notices it has clock offset in
        excess of --max-offset, it will commit suicide. Setting this value too
        high may decrease transaction performance in the presence of
        contention.
`,
	"memtable-budget": `
        Total size in bytes for memtables, shared evenly if there are multiple
        storage devices.
`,
	"time-until-store-dead": `
		Adjusts the timeout for stores.  If there's been no gossiped updated
		from a store after this time, the store is considered unavailable.
        Replicas on an unavailable store will be moved to available ones.
`,
	"stores": `
        A comma-separated list of stores, specified by a colon-separated list
        of device attributes followed by '=' and either a filepath for a
        persistent store or an integer size in bytes for an in-memory
        store. Device attributes typically include whether the store is flash
        (ssd), spinny disk (hdd), fusion-io (fio), in-memory (mem); device
        attributes might also include speeds and other specs (7200rpm,
        200kiops, etc.). For example:

          --stores=hdd:7200rpm=/mnt/hda1,ssd=/mnt/ssd01,ssd=/mnt/ssd02,mem=1073741824.
`,
}

func normalizeStdFlagName(s string) string {
	return strings.Replace(s, "_", "-", -1)
}

// initFlags sets the server.Context values to flag values.
// Keep in sync with "server/context.go". Values in Context should be
// settable here.
func initFlags(ctx *server.Context) {
	// Map any flags registered in the standard "flag" package into the
	// top-level cockroach command.
	pf := dmatrixCmd.PersistentFlags()
	flag.VisitAll(func(f *flag.Flag) {
		pf.Var(pflagValue{f.Value}, normalizeStdFlagName(f.Name), f.Usage)
	})

	{
		f := startCmd.Flags()

		// Server flags.
		f.StringVar(&ctx.Addr, "addr", ctx.Addr, flagUsage["addr"])
		f.StringVar(&ctx.Attrs, "attrs", ctx.Attrs, flagUsage["attrs"])
		f.StringVar(&ctx.Stores, "stores", ctx.Stores, flagUsage["stores"])
		f.DurationVar(&ctx.MaxOffset, "max-offset", ctx.MaxOffset, flagUsage["max-offset"])

		// Gossip flags.
		f.StringVar(&ctx.GossipBootstrap, "gossip", ctx.GossipBootstrap, flagUsage["gossip"])

		// Engine flags.
		f.Int64Var(&ctx.CacheSize, "cache-size", ctx.CacheSize, flagUsage["cache-size"])
		f.Int64Var(&ctx.MemtableBudget, "memtable-budget", ctx.MemtableBudget, flagUsage["memtable-budget"])

		if err := startCmd.MarkFlagRequired("gossip"); err != nil {
			panic(err)
		}
		if err := startCmd.MarkFlagRequired("stores"); err != nil {
			panic(err)
		}
	}

	{
		f := exterminateCmd.Flags()
		f.StringVar(&ctx.Stores, "stores", ctx.Stores, flagUsage["stores"])
		if err := exterminateCmd.MarkFlagRequired("stores"); err != nil {
			panic(err)
		}
	}

	clientCmds := []*cobra.Command{
		kvCmd, exterminateCmd, quitCmd, /* startCmd is covered above */
	}
	for _, cmd := range clientCmds {
		f := cmd.PersistentFlags()
		f.StringVar(&ctx.Addr, "addr", ctx.Addr, flagUsage["addr"])
	}
}

func init() {
	initFlags(context)
}
