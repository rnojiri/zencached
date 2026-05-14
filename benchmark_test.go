package zencached_test

import (
	"context"
	"testing"
	"time"

	"github.com/rnojiri/zencached"
)

func Benchmark(b *testing.B) {

	nodes := startMemcachedCluster()
	defer terminatePods()

	c := &zencached.Configuration{
		Nodes:                     nodes,
		NumConnectionsPerNode:     10,
		TelnetConfiguration:       *createTelnetConf(nil),
		NumNodeListRetries:        1,
		RebalanceOnDisconnection:  false,
		ZencachedMetricsCollector: nil,
		NodeListFunction:          nil,
		NodeListRetryTimeout:      time.Second,
	}

	z, err := zencached.New(c)
	if err != nil {
		panic(err)
	}

	path := []byte("benchmark")
	key := []byte("benchmark")
	value := []byte("benchmark")
	route := []byte{0}
	ctx := context.Background()

	for n := 0; n < b.N; n++ {
		_, err := z.Set(ctx, route, path, key, value, defaultTTL)
		if err != nil {
			panic(err)
		}
		_, err = z.Get(ctx, route, path, key)
		if err != nil {
			panic(err)
		}
	}
}
