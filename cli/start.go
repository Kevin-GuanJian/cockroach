package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cockroachdb/cockroach/util"
	"github.com/cockroachdb/cockroach/util/log"
	"github.com/cockroachdb/cockroach/util/stop"
	"github.com/dmatrixdb/dmatrix/client"
	"github.com/dmatrixdb/dmatrix/security"
	"github.com/dmatrixdb/dmatrix/server"
	"github.com/dmatrixdb/dmatrix/storage/engine"

	"github.com/spf13/cobra"
)

// Context is the CLI Context used for the server.
var context = server.NewContext()

var startCmd = &cobra.Command{
	Use:     "start",
	Short:   "start a node",
	Long:    `start a node`,
	Example: `  dmatrix start -stores=ssd=/mnt/ssd1,...`,
	Run:     runStart,
}

func runStart(_ *cobra.Command, _ []string) {
	info := util.GetBuildInfo()
	log.Infof("build Vers: %s", info.Vers)
	log.Infof("build Tag:  %s", info.Tag)
	log.Infof("build Time: %s", info.Time)
	log.Infof("build Deps: %s", info.Deps)

	// Default user for servers.
	context.User = security.NodeUser

	stopper := stop.NewStopper()

	if err := context.InitStores(stopper); err != nil {
		log.Errorf("failed to initialize stores: %s", err)
		return
	}

	if err := context.InitNode(); err != nil {
		log.Errorf("failed to initialize node: %s", err)
		return
	}

	log.Info("starting dmatrix server")
	s, err := server.NewServer(context, stopper)
	if err != nil {
		log.Errorf("failed to start Cockroach server: %s", err)
		return
	}

	if err := s.Start(); err != nil {
		log.Errorf("cockroach server exited with error: %s", err)
		return
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, os.Kill)
	// TODO(spencer): move this behind a build tag.
	signal.Notify(signalCh, syscall.SIGTERM)

	// Block until one of the signals above is received or the stopper
	// is stopped externally (for example, via the quit endpoint).
	select {
	case <-stopper.ShouldStop():
	case <-signalCh:
		go s.Stop()
	}

	log.Info("initiating graceful shutdown of server")

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if log.V(1) {
					log.Infof("running tasks:\n%s", stopper.RunningTasks())
				}
				log.Infof("%d running tasks", stopper.NumTasks())
			case <-stopper.ShouldStop():
				return
			}
		}
	}()

	select {
	case <-signalCh:
		log.Warningf("second signal received, initiating hard shutdown")
	case <-time.After(time.Minute):
		log.Warningf("time limit reached, initiating hard shutdown")
	case <-stopper.IsStopped():
		log.Infof("server drained and shutdown completed")
	}
	log.Flush()
}

var exterminateCmd = &cobra.Command{
	Use:   "exterminate",
	Short: "destroy all data held by the node",
	Long:  `destroy all data held by the node`,
	Run:   runExterminate,
}

func runExterminate(_ *cobra.Command, _ []string) {
	stopper := stop.NewStopper()
	defer stopper.Stop()
	if err := context.InitStores(stopper); err != nil {
		log.Errorf("failed to initialize context: %s", err)
		return
	}

	// First attempt to shutdown the server. Note that an error of EOF just
	// means the HTTP server shutdown before the request to quit returned.
	admin := client.NewAdminClient(&context.Context, context.Addr, client.Quit)
	body, err := admin.Get()
	if err != nil {
		log.Infof("shutdown node %s: %s", context.Addr, err)
	} else {
		log.Infof("shutdown node in anticipation of data extermination: %s", body)
	}

	// Exterminate all data held in specified stores.
	for _, e := range context.Engines {
		if rocksdb, ok := e.(*engine.RocksDB); ok {
			log.Infof("exterminating data from store %s", e)
			if err := rocksdb.Destroy(); err != nil {
				log.Errorf("unable to destroy store %s: %s", e, err)
				osExit(1)
			}
		}
	}
	log.Infof("exterminated all data from stores %s", context.Engines)
}

var quitCmd = &cobra.Command{
	Use:   "quit",
	Short: "drain and shutdown node\n",
	Long: `
Shutdown the server. The first stage is drain, where any new requests
will be ignored by the server. When all extant requests have been
completed, the server exits.
`,
	Run: runQuit,
}

func runQuit(_ *cobra.Command, _ []string) {
	admin := client.NewAdminClient(&context.Context, context.Addr, client.Quit)
	body, err := admin.Get()
	if err != nil {
		fmt.Printf("shutdown node error: %s\n", err)
		osExit(1)
		return
	}
	fmt.Printf("node drained and shutdown: %s\n", body)
}
