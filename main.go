package main

import (
	"encoding/json"
	"flag"
	"github.com/kahgeh/whale-disco/pkg/mappers"
	"github.com/kahgeh/whale-disco/pkg/registry/whale"
	"strconv"
	"time"

	"github.com/kahgeh/whale-disco/pkg/ctx"
	"github.com/kahgeh/whale-disco/pkg/logger"
	"github.com/kahgeh/whale-disco/pkg/server"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	testv3 "github.com/envoyproxy/go-control-plane/pkg/test/v3"
)

var (
	verbose    bool
	port       uint
	domainName string
	nodeID     string
)

func init() {
	flag.StringVar(&domainName, "domain", "*", "domain name for routes")
	flag.BoolVar(&verbose, "verbose", false, "detailed log level")
	// The port that this xDS server listens on
	flag.UintVar(&port, "port", 18000, "xDS management server port")
	// Tell Envoy to use this Node ID
	flag.StringVar(&nodeID, "nodeID", "test-id", "Node ID")
}

func initLog(verbose bool) {
	level := logger.NormalLogLevel
	if verbose {
		level = logger.DebugLogLevel
	}
	logger.Initialise(level)
}

func main() {
	flag.Parse()
	initLog(verbose)
	log := logger.New("runWhaleDisco")
	defer log.LogDone()

	defer logger.Sync()
	defer ctx.CleanUp()
	go ctx.WaitOnCtrlCSignalOrCompletion()

	// Create a cache
	cache := cachev3.NewSnapshotCache(false, cachev3.IDHash{}, log)

	// RunInBackground the xDS server
	cb := &testv3.Callbacks{Debug: verbose}
	srv := serverv3.NewServer(ctx.GetContext(), cache, cb)
	go server.RunServer(ctx.GetContext(), srv, port)
	dockerRegistry := whale.New()
	updateChannel := dockerRegistry.Run()
	appContext := ctx.GetContext()
	var previousUpdateHash uint32
	version := 1

	for {
		select {
		case update := <-updateChannel:
			if update.GetHash() != previousUpdateHash {
				log.Info("different version detected, updating snapshot...")
				clusterEndpoints := update.GroupByCluster()
				v, _ := json.Marshal(clusterEndpoints)
				log.Info("discovered", string(v))
				version = version + 1
				newSnapshot, err := mappers.MapToSnapshot(clusterEndpoints, strconv.Itoa(version), domainName)
				if err != nil {
					log.Warnf("Skip update because %s", err.Error())
					continue
				}
				if err := cache.SetSnapshot(nodeID, newSnapshot); err != nil {
					log.Failf("snapshot error %q for %+v", err, newSnapshot)
				}
				previousUpdateHash = update.GetHash()
				log.Infof("config replaced with version %v", version)
			}
		case <-appContext.Done():
			return
		}
		<-time.After(5 * time.Second)
	}

}
