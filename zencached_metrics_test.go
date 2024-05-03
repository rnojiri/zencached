package zencached_test

import (
	"testing"

	"github.com/rnojiri/dockerh"
	"github.com/rnojiri/logh"
	"github.com/rnojiri/zencached"
	"github.com/stretchr/testify/suite"
)

const (
	memcachedMetricsPodName string = "memcached-metrics-pod"
	memcachedMetricsPodPort int    = 11211
)

type metricsCollector struct {
	numNodeDistributionEvent          int
	numNodeConnectionAvailabilityTime int
	numCommandExecutionElapsedTime    int
	numCommandExecution               int
	numCommandExecutionError          int
	numCacheMissEvent                 int
	numCacheHitEvent                  int
}

// NodeDistributionEvent - signalizes a node distribution event
func (mc *metricsCollector) NodeDistributionEvent(node string) {

	mc.numNodeDistributionEvent++
}

// NodeConnectionAvailabilityTime - the elapsed time waiting for an available connection
func (mc *metricsCollector) NodeConnectionAvailabilityTime(node string, elapsedTime int64) {

	mc.numNodeConnectionAvailabilityTime++
}

// CommandExecutionElapsedTime - command execution elapsed time
func (mc *metricsCollector) CommandExecutionElapsedTime(node string, operation, path, key []byte, elapsedTime int64) {

	mc.numCommandExecutionElapsedTime++
}

// CommandExecution - an memcached command event
func (mc *metricsCollector) CommandExecution(node string, operation, path, key []byte) {

	mc.numCommandExecution++
}

// CacheMissEvent - signalizes a cache miss event
func (mc *metricsCollector) CacheMissEvent(node string, operation, path, key []byte) {

	mc.numCacheMissEvent++
}

// CacheHitEvent - signalizes a cache hit event
func (mc *metricsCollector) CacheHitEvent(node string, operation, path, key []byte) {

	mc.numCacheHitEvent++
}

func (mc *metricsCollector) CommandExecutionError(node string, operation, path, key []byte, err error) {

	mc.numCommandExecutionError++
}

func (mc *metricsCollector) zero() {

	mc.numNodeDistributionEvent = 0
	mc.numNodeConnectionAvailabilityTime = 0
	mc.numCommandExecutionElapsedTime = 0
	mc.numCommandExecution = 0
	mc.numCacheMissEvent = 0
	mc.numCacheHitEvent = 0
	mc.numCommandExecutionError = 0
}

type zencachedMetricsTestSuite struct {
	suite.Suite
	instance *zencached.Zencached
	metrics  *metricsCollector
}

func (ts *zencachedMetricsTestSuite) SetupSuite() {

	logh.ConfigureGlobalLogger(logh.DEBUG, logh.CONSOLE)

	terminatePods()

	host, err := dockerh.CreateMemcached(memcachedMetricsPodName, memcachedMetricsPodPort, 64)
	if err != nil {
		ts.T().Fatalf("expected no errors creating memcached metrics pod: %v", err)
	}

	ts.metrics = &metricsCollector{}

	ts.instance, _, err = createZencached([]zencached.Node{{Host: host, Port: memcachedMetricsPodPort}}, false, ts.metrics)
	if err != nil {
		ts.T().Fatalf("expected no errors creating zencached: %v", err)
	}
}

func (ts *zencachedMetricsTestSuite) SetupTest() {

	ts.metrics.zero()
}

func (ts *zencachedMetricsTestSuite) TearDownSuite() {

	ts.instance.Shutdown()

	terminatePods()
}

func (ts *zencachedMetricsTestSuite) TestNodeDistributionEvents() {

	ts.instance.Get(nil, []byte("path1"), []byte("key1"))

	ts.Equal(1, ts.metrics.numNodeDistributionEvent, "expects one node distribution event")
	ts.Equal(1, ts.metrics.numNodeConnectionAvailabilityTime, "expects one node connection availability time measurement")
}

func (ts *zencachedMetricsTestSuite) TestCommandExecutionEvents() {

	ts.instance.Add(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
	ts.instance.Set(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
	ts.instance.Get(nil, []byte("path1"), []byte("key1"))
	ts.instance.Delete(nil, []byte("path1"), []byte("key1"))

	ts.Equal(4, ts.metrics.numCommandExecution, "expects four command execution events")
	ts.Equal(4, ts.metrics.numCommandExecutionElapsedTime, "expects four command execution time measurements")
}

func (ts *zencachedMetricsTestSuite) TestCacheMissEvents() {

	ts.instance.Get(nil, []byte("path1"), []byte("key1"))
	ts.instance.Get([]byte("hash"), []byte("path2"), []byte("key2"))
	ts.instance.ClusterGet([]byte("path3"), []byte("key3"))

	ts.Equal(3, ts.metrics.numCacheMissEvent, "expects three cache miss events")
}

func (ts *zencachedMetricsTestSuite) TestCacheHitEvents() {

	ts.instance.Set(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
	ts.instance.Get(nil, []byte("path1"), []byte("key1"))
	ts.instance.ClusterGet([]byte("path1"), []byte("key1"))
	ts.instance.Delete(nil, []byte("path1"), []byte("key1"))

	ts.Equal(3, ts.metrics.numCacheHitEvent, "expects two cache hit events")
}

// TestZCacheHitEvents - executes last because of the 'Z'
func (ts *zencachedMetricsTestSuite) TestZCommandExecutionError() {

	err := dockerh.Remove(memcachedMetricsPodName)
	if !ts.NoError(err, "expected no errors removing memcached metrics pod: %v", err) {
		return
	}

	ts.instance.Add(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
	ts.instance.Get(nil, []byte("path1"), []byte("key1"))
	ts.instance.ClusterGet([]byte("path1"), []byte("key1"))
	ts.instance.Delete(nil, []byte("path1"), []byte("key1"))

	ts.Equal(8, ts.metrics.numCommandExecutionError, "expects four command execution error events")
}

func TestZencachedMetricsSuite(t *testing.T) {

	suite.Run(t, new(zencachedMetricsTestSuite))
}
