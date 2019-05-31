package binaries

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/onsi/gomega/gexec"
)

type BuildPaths struct {
	Router            string `json:"router"`
	TrafficController string `json:"traffic_controller"`
	RLP               string `json:"rlp"`
	RLPGateway        string `json:"rlp_gateway"`
}

func (bp BuildPaths) Marshal() ([]byte, error) {
	return json.Marshal(bp)
}

func (bp *BuildPaths) Unmarshal(text []byte) error {
	return json.Unmarshal(text, bp)
}

func (bp BuildPaths) SetEnv() {
	os.Setenv("ROUTER_BUILD_PATH", bp.Router)
	os.Setenv("TRAFFIC_CONTROLLER_BUILD_PATH", bp.TrafficController)
	os.Setenv("RLP_BUILD_PATH", bp.RLP)
	os.Setenv("RLP_GATEWAY_BUILD_PATH", bp.RLPGateway)
}

func Build() (BuildPaths, chan error) {
	var bp BuildPaths
	errors := make(chan error, 100)
	defer close(errors)

	if os.Getenv("SKIP_BUILD") != "" {
		fmt.Println("Skipping building of binaries")
		bp.Router = os.Getenv("ROUTER_BUILD_PATH")
		bp.TrafficController = os.Getenv("TRAFFIC_CONTROLLER_BUILD_PATH")
		bp.RLP = os.Getenv("RLP_BUILD_PATH")
		bp.RLPGateway = os.Getenv("RLP_GATEWAY_BUILD_PATH")
		return bp, errors
	}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	wg.Add(4)

	go func() {
		defer wg.Done()
		path, err := gexec.Build("code.cloudfoundry.org/loggregator/router", "-race")
		if err != nil {
			errors <- err
			return
		}
		mu.Lock()
		defer mu.Unlock()
		bp.Router = path
	}()

	go func() {
		defer wg.Done()
		path, err := gexec.Build("code.cloudfoundry.org/loggregator/trafficcontroller", "-race")
		if err != nil {
			errors <- err
			return
		}
		mu.Lock()
		defer mu.Unlock()
		bp.TrafficController = path
	}()

	go func() {
		defer wg.Done()
		path, err := gexec.Build("code.cloudfoundry.org/loggregator/rlp", "-race")
		if err != nil {
			errors <- err
			return
		}
		mu.Lock()
		defer mu.Unlock()
		bp.RLP = path
	}()

	go func() {
		defer wg.Done()
		path, err := gexec.Build("code.cloudfoundry.org/loggregator/rlp-gateway", "-race")
		if err != nil {
			errors <- err
			return
		}
		mu.Lock()
		defer mu.Unlock()
		bp.RLPGateway = path
	}()

	wg.Wait()
	return bp, errors
}

func Cleanup() {
	gexec.CleanupBuildArtifacts()
}
