package zencached_test

import (
	"testing"
	"time"

	"github.com/rnojiri/zencached"
)

func Benchmark(b *testing.B) {

	nodes := startMemcachedCluster()
	defer terminatePods()

	c := &zencached.Configuration{
		Nodes:                      nodes,
		NumConnectionsPerNode:      10,
		TelnetConfiguration:        *createTelnetConf(nil),
		CommandExecutionBufferSize: 100,
		NumNodeListRetries:         1,
		RebalanceOnDisconnection:   false,
		ZencachedMetricsCollector:  nil,
		NodeListFunction:           nil,
		NodeListRetryTimeout:       time.Second,
	}

	z, err := zencached.New(c)
	if err != nil {
		panic(err)
	}

	path := []byte("benchmark")
	key := []byte("benchmark")
	value := []byte("benchmark")
	route := []byte{0}

	for n := 0; n < b.N; n++ {
		_, err := z.Set(route, path, key, value, defaultTTL)
		if err != nil {
			panic(err)
		}
		_, _, err = z.Get(route, path, key)
		if err != nil {
			panic(err)
		}
	}
}
