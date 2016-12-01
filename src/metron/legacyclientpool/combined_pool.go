package legacyclientpool

import (
	"errors"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
)

type Pool interface {
	Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) error
}

type CombinedPool struct {
	pools []Pool
}

func NewCombinedPool(pools ...Pool) *CombinedPool {
	return &CombinedPool{
		pools: pools,
	}
}

func (p *CombinedPool) Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) error {
	for _, pool := range p.pools {
		err := pool.Write(message, chainers...)
		if err == nil {
			return nil
		}
	}

	return errors.New("unable to write to any of the pools")
}
