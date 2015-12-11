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
// Author: Marc Berhault (marc@cockroachlabs.com)

package base

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"
)

// Base context defaults.
const (
	rpcScheme   = "rpc"
	rpcsScheme  = "rpcs"
	httpScheme  = "http"
	httpsScheme = "https"

	// NetworkTimeout is the timeout used for network operations.
	NetworkTimeout = 3 * time.Second
)

// Context is embedded by server.Context. A base context is not meant to be
// used directly, but embedding contexts should call ctx.InitDefaults().
type Context struct {
	// User running this process. It could be the user under which
	// the server is running ("node"), or the user passed in client calls.
	User string
	// Protects both clientTLSConfig and serverTLSConfig.
	tlsConfigMu sync.Mutex
	// clientTLSConfig is the loaded client tlsConfig. It is initialized lazily.
	clientTLSConfig *tls.Config
	// serverTLSConfig is the loaded server tlsConfig. It is initialized lazily.
	serverTLSConfig *tls.Config

	// httpClient is a lazily-initialized http client.
	// It should be accessed through Context.GetHTTPClient() which will
	// initialize if needed.
	httpClient *http.Client
	// Protects httpClient.
	httpClientMu sync.Mutex
}

// InitDefaults sets up the default values for a context.
func (ctx *Context) InitDefaults() {

}

// RPCRequestScheme returns "rpc" or "rpcs" based on the value of Insecure.
func (ctx *Context) RPCRequestScheme() string {
	return rpcsScheme
}

// HTTPRequestScheme returns "http" or "https" based on the value of Insecure.
func (ctx *Context) HTTPRequestScheme() string {
	return httpsScheme
}

// GetHTTPClient returns the context http client, initializing it
// if needed. It uses the context client TLS config.
func (ctx *Context) GetHTTPClient() (*http.Client, error) {
	ctx.httpClientMu.Lock()
	defer ctx.httpClientMu.Unlock()

	if ctx.httpClient != nil {
		return ctx.httpClient, nil
	}

	ctx.httpClient = &http.Client{
		Transport: &http.Transport{},
		Timeout:   NetworkTimeout,
	}

	return ctx.httpClient, nil
}
