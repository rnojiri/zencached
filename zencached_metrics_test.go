package zencached_test

import (
	"errors"
	"sync"
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
	numCommandExecutionElapsedTime     int
	numCommandExecution                int
	numCommandExecutionError           int
	numCacheMissEvent                  int
	numCacheHitEvent                   int
	numNodeRebalanceEvent              int
	numNodeListingEvent                int
	numNodeListingError                int
	numNodeListingElapsedTime          int
	numNodeRebalanceElapsedTime        int
	numResourcesChangeEvent            int
	numNoResourcesAvailableEvents      int
	numAvailableResourcesRestoredEvent int
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

func (mc *metricsCollector) NodeRebalanceEvent(numNodes int) {

	mc.numNodeRebalanceEvent++
}

func (mc *metricsCollector) NodeListingEvent(numNodes int) {

	mc.numNodeListingEvent++
}

func (mc *metricsCollector) NodeListingError() {

	mc.numNodeListingError++
}

func (mc *metricsCollector) NodeListingElapsedTime(elapsedTime int64) {

	mc.numNodeListingElapsedTime++
}

func (mc *metricsCollector) NodeRebalanceElapsedTime(elapsedTime int64) {

	mc.numNodeRebalanceElapsedTime++
}

func (mc *metricsCollector) NumResourcesChangeEvent(node string, numResources int) {

	mc.numResourcesChangeEvent++
}

func (mc *metricsCollector) NoAvailableResourcesEvent(node string) {

	mc.numNoResourcesAvailableEvents++
}

func (mc *metricsCollector) AvailableResourcesRestoredEvent(node string) {

	mc.numAvailableResourcesRestoredEvent++
}

func (mc *metricsCollector) reset() {

	mc.numCommandExecutionElapsedTime = 0
	mc.numCommandExecution = 0
	mc.numCacheMissEvent = 0
	mc.numCacheHitEvent = 0
	mc.numCommandExecutionError = 0
	mc.numNodeRebalanceEvent = 0
	mc.numNodeListingEvent = 0
	mc.numNodeListingError = 0
	mc.numNodeListingElapsedTime = 0
	mc.numNodeRebalanceElapsedTime = 0
	mc.numResourcesChangeEvent = 0
	mc.numNoResourcesAvailableEvents = 0
	mc.numAvailableResourcesRestoredEvent = 0
}

type zencachedMetricsTestSuite struct {
	suite.Suite
	instance                   *zencached.Zencached
	config                     *zencached.Configuration
	metrics                    *metricsCollector
	telnetMetrics              *telnetMetricsCollector
	commandExecutionBufferSize uint32
	memcachedHost              string
}

func (ts *zencachedMetricsTestSuite) SetupSuite() {

	logh.ConfigureGlobalLogger(logh.DEBUG, logh.CONSOLE)

	terminatePods()

	var err error
	ts.memcachedHost, err = dockerh.CreateMemcached(memcachedMetricsPodName, memcachedMetricsPodPort, 64)
	if err != nil {
		ts.T().Fatalf("expected no errors creating memcached metrics pod: %v", err)
	}

	ts.metrics = &metricsCollector{}
	ts.telnetMetrics = &telnetMetricsCollector{}

	if ts.commandExecutionBufferSize == 0 {
		ts.commandExecutionBufferSize = 10
	}

	ts.instance, ts.config, err = createZencached(
		[]zencached.Node{{Host: ts.memcachedHost, Port: memcachedMetricsPodPort}},
		ts.commandExecutionBufferSize, false,
		ts.metrics, ts.telnetMetrics,
		zencached.CompressionTypeNone,
	)
	if err != nil {
		ts.T().Fatalf("expected no errors creating zencached: %v", err)
	}
}

func (ts *zencachedMetricsTestSuite) SetupTest() {

	ts.metrics.reset()
	ts.telnetMetrics.reset()
}

func (ts *zencachedMetricsTestSuite) TearDownSuite() {

	ts.instance.Shutdown()

	terminatePods()
}

type zencachedCommonMetricsTestSuite struct {
	zencachedMetricsTestSuite
}

func (ts *zencachedCommonMetricsTestSuite) TestCommandExecutionEvents() {

	ts.instance.Add(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
	ts.instance.SendTimedMetrics()
	ts.instance.Set(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
	ts.instance.SendTimedMetrics()
	ts.instance.Get(nil, []byte("path1"), []byte("key1"))
	ts.instance.SendTimedMetrics()
	ts.instance.Delete(nil, []byte("path1"), []byte("key1"))
	ts.instance.SendTimedMetrics()

	ts.Equal(4, ts.metrics.numCommandExecution, "expects four command execution events")
	ts.Equal(4, ts.metrics.numCommandExecutionElapsedTime, "expects four command execution time measurements")
	ts.Equal(4, ts.metrics.numResourcesChangeEvent, "expected four events changing the number of resources")
	ts.Equal(0, ts.telnetMetrics.numResolveAddressElapsedTime, "expected a resolve address event")
	ts.Equal(0, ts.telnetMetrics.numDialElapsedTime, "expected a dial event")
	ts.Equal(4, ts.telnetMetrics.numSendElapsedTime, "expected a send event")
	ts.Equal(4, ts.telnetMetrics.numWriteElapsedTime, "expected a write event")
	ts.Equal(4, ts.telnetMetrics.numReadElapsedTime, "expected a read event")
	ts.Equal(0, ts.telnetMetrics.numCloseElapsedTime, "expected a close event")
}

func (ts *zencachedCommonMetricsTestSuite) TestCacheMissEvents() {

	ts.instance.Get(nil, []byte("path1"), []byte("key1"))
	ts.instance.SendTimedMetrics()
	ts.instance.Get([]byte("hash"), []byte("path2"), []byte("key2"))
	ts.instance.SendTimedMetrics()
	ts.instance.ClusterGet([]byte("path3"), []byte("key3"))
	ts.instance.SendTimedMetrics()

	ts.Equal(3, ts.metrics.numCacheMissEvent, "expects three cache miss events")
	ts.Equal(3, ts.metrics.numResourcesChangeEvent, "expected three events changing the number of resources")
	ts.Equal(3, ts.telnetMetrics.numSendElapsedTime, "expected a send event")
	ts.Equal(3, ts.telnetMetrics.numWriteElapsedTime, "expected a write event")
	ts.Equal(3, ts.telnetMetrics.numReadElapsedTime, "expected a read event")
	ts.Equal(0, ts.telnetMetrics.numCloseElapsedTime, "expected a close event")
}

func (ts *zencachedCommonMetricsTestSuite) TestCacheHitEvents() {

	ts.instance.Set(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
	ts.instance.SendTimedMetrics()
	ts.instance.Get(nil, []byte("path1"), []byte("key1"))
	ts.instance.SendTimedMetrics()
	ts.instance.ClusterGet([]byte("path1"), []byte("key1"))
	ts.instance.SendTimedMetrics()
	ts.instance.Delete(nil, []byte("path1"), []byte("key1"))
	ts.instance.SendTimedMetrics()

	ts.Equal(2, ts.metrics.numCacheHitEvent, "expects two cache hit events")
	ts.Equal(4, ts.metrics.numResourcesChangeEvent, "expected four events changing the number of resources")
	ts.Equal(4, ts.telnetMetrics.numSendElapsedTime, "expected a send event")
	ts.Equal(4, ts.telnetMetrics.numWriteElapsedTime, "expected a write event")
	ts.Equal(4, ts.telnetMetrics.numReadElapsedTime, "expected a read event")
	ts.Equal(0, ts.telnetMetrics.numCloseElapsedTime, "expected a close event")
}

// TestZCacheHitEvents - executes last because of the 'Z'
func (ts *zencachedCommonMetricsTestSuite) TestZ1CommandExecutionError() {

	err := dockerh.Remove(memcachedMetricsPodName)
	if !ts.NoError(err, "expected no errors removing memcached metrics pod: %v", err) {
		return
	}

	ts.instance.Add(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
	ts.instance.SendTimedMetrics()
	ts.instance.Get(nil, []byte("path1"), []byte("key1"))
	ts.instance.SendTimedMetrics()
	ts.instance.ClusterGet([]byte("path1"), []byte("key1"))
	ts.instance.SendTimedMetrics()
	ts.instance.Delete(nil, []byte("path1"), []byte("key1"))
	ts.instance.SendTimedMetrics()

	ts.Equal(4, ts.metrics.numCommandExecutionError, "expects four command execution error events")
	ts.Equal(4, ts.metrics.numResourcesChangeEvent, "expected eight events changing the number of resources")
	ts.Equal(4, ts.telnetMetrics.numSendElapsedTime, "expected a send event")
	ts.Equal(4, ts.telnetMetrics.numWriteElapsedTime, "expected a write event")
	ts.Equal(4, ts.telnetMetrics.numReadElapsedTime, "expected a read event")
	ts.Equal(0, ts.telnetMetrics.numCloseElapsedTime, "expected a close event")
}

func (ts *zencachedCommonMetricsTestSuite) TestZ2RebalanceMetrics() {

	ts.instance.Rebalance()
	ts.instance.SendTimedMetrics()

	ts.Equal(1, ts.metrics.numNodeRebalanceEvent, "expected a rebalance event")
	ts.Equal(1, ts.metrics.numNodeRebalanceElapsedTime, "expected a rebalance elapsed time event")
	ts.Equal(1, ts.metrics.numNodeListingEvent, "expected a node listing event")
	ts.Equal(1, ts.metrics.numNodeListingElapsedTime, "expected a node listing elapsed time event")
	ts.Equal(0, ts.metrics.numNodeListingError, "expected no node listing errors")
	ts.Equal(0, ts.telnetMetrics.numSendElapsedTime, "expected a send event")
	ts.Equal(0, ts.telnetMetrics.numWriteElapsedTime, "expected a write event")
	ts.Equal(0, ts.telnetMetrics.numReadElapsedTime, "expected a read event")
	ts.Equal(0, ts.telnetMetrics.numCloseElapsedTime, "expected a close event")
}

func (ts *zencachedCommonMetricsTestSuite) TestZ3RebalanceMetricsError() {

	ts.config.NodeListFunction = func() ([]zencached.Node, error) {
		return []zencached.Node{
			{
				Host: ts.memcachedHost,
				Port: memcachedMetricsPodPort,
			},
			{
				Host: "127.0.0.1",
				Port: 12345,
			},
		}, nil
	}

	ts.instance.Rebalance()
	ts.instance.SendTimedMetrics()

	ts.Equal(2, ts.metrics.numNodeRebalanceEvent, "expected two rebalance events")
	ts.Equal(2, ts.metrics.numNodeRebalanceElapsedTime, "expected two rebalance elapsed time events")
	ts.Equal(1, ts.metrics.numNodeListingEvent, "expected a node listing event")
	ts.Equal(1, ts.metrics.numNodeListingElapsedTime, "expected a node listing elapsed time event")
	ts.Equal(0, ts.metrics.numNodeListingError, "expected no node listing errors")
	ts.Equal(0, ts.telnetMetrics.numSendElapsedTime, "expected a send event")
	ts.Equal(0, ts.telnetMetrics.numWriteElapsedTime, "expected a write event")
	ts.Equal(0, ts.telnetMetrics.numReadElapsedTime, "expected a read event")
}

func (ts *zencachedCommonMetricsTestSuite) TestZ4NodeListingErrorMetricsError() {

	ts.config.NodeListFunction = func() ([]zencached.Node, error) {
		return nil, errors.New("some error")
	}

	ts.instance.Rebalance()
	ts.instance.SendTimedMetrics()

	ts.Equal(2, ts.metrics.numNodeRebalanceEvent, "expected two rebalance events")
	ts.Equal(2, ts.metrics.numNodeRebalanceElapsedTime, "expected two rebalance elapsed time events")
	ts.Equal(1, ts.metrics.numNodeListingEvent, "expected a node listing event")
	ts.Equal(1, ts.metrics.numNodeListingElapsedTime, "expected a node listing elapsed time event")
	ts.Equal(3, ts.metrics.numNodeListingError, "expected three node listing errors")
	ts.Equal(0, ts.telnetMetrics.numSendElapsedTime, "expected a send event")
	ts.Equal(0, ts.telnetMetrics.numWriteElapsedTime, "expected a write event")
	ts.Equal(0, ts.telnetMetrics.numReadElapsedTime, "expected a read event")
	ts.Equal(0, ts.telnetMetrics.numCloseElapsedTime, "expected a close event")
}

func TestZencachedCommonMetricsSuite(t *testing.T) {

	suite.Run(t, new(zencachedCommonMetricsTestSuite))
}

type zencachedNoResourceMetricsTestSuite struct {
	zencachedMetricsTestSuite
}

func (ts *zencachedNoResourceMetricsTestSuite) TestNoResoucesAvailableEvents() {

	wc := sync.WaitGroup{}

	for i := 0; i < 3; i++ {
		wc.Add(1)
		go func() {
			ts.instance.Add(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
			ts.instance.SendTimedMetrics()
			wc.Done()
		}()
	}

	wc.Wait()

	ts.Equal(1, ts.metrics.numCommandExecution, "expects one command execution event")
	ts.Equal(1, ts.metrics.numCommandExecutionElapsedTime, "expects one command execution time measurement")
	ts.Equal(3, ts.metrics.numResourcesChangeEvent, "expected three event changing the number of resources")
	ts.Equal(1, ts.metrics.numNoResourcesAvailableEvents, "expected one no resource available events")
	ts.Equal(1, ts.metrics.numAvailableResourcesRestoredEvent, "expected one resource available events")
	ts.Equal(0, ts.telnetMetrics.numResolveAddressElapsedTime, "expected a resolve address event")
	ts.Equal(0, ts.telnetMetrics.numDialElapsedTime, "expected a dial event")
	ts.Equal(1, ts.telnetMetrics.numSendElapsedTime, "expected a send event")
	ts.Equal(1, ts.telnetMetrics.numWriteElapsedTime, "expected a write event")
	ts.Equal(1, ts.telnetMetrics.numReadElapsedTime, "expected a read event")
	ts.Equal(0, ts.telnetMetrics.numCloseElapsedTime, "expected a close event")
}

func (ts *zencachedNoResourceMetricsTestSuite) TestSequentialCallsShouldNeverExhaustResources() {

	for i := 0; i < 10; i++ {
		ts.instance.Add(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
		ts.instance.SendTimedMetrics()
		ts.instance.Get(nil, []byte("path1"), []byte("key1"))
		ts.instance.SendTimedMetrics()
		ts.instance.ClusterGet([]byte("path1"), []byte("key1"))
		ts.instance.SendTimedMetrics()
		ts.instance.Delete(nil, []byte("path1"), []byte("key1"))
		ts.instance.SendTimedMetrics()
	}

	ts.Equal(40, ts.metrics.numCommandExecution, "expects fourty command execution events")
	ts.Equal(40, ts.metrics.numCommandExecutionElapsedTime, "expects fourty command execution time measurements")
	ts.Equal(40, ts.metrics.numResourcesChangeEvent, "expected fourty event changing the number of resources")
	ts.Equal(0, ts.metrics.numNoResourcesAvailableEvents, "expected zero no resource available events")
	ts.Equal(0, ts.metrics.numAvailableResourcesRestoredEvent, "expected zero resource available events")
	ts.Equal(0, ts.telnetMetrics.numResolveAddressElapsedTime, "expected a resolve address event")
	ts.Equal(0, ts.telnetMetrics.numDialElapsedTime, "expected a dial event")
	ts.Equal(40, ts.telnetMetrics.numSendElapsedTime, "expected fourty send events")
	ts.Equal(40, ts.telnetMetrics.numWriteElapsedTime, "expected fourty write events")
	ts.Equal(40, ts.telnetMetrics.numReadElapsedTime, "expected fourty read events")
	ts.Equal(0, ts.telnetMetrics.numCloseElapsedTime, "expected a close event")
}

func (ts *zencachedNoResourceMetricsTestSuite) TestResoucesAvailableEvents() {

	wc := sync.WaitGroup{}

	for i := 0; i < 2; i++ {
		wc.Add(1)
		go func() {
			ts.instance.Add(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
			ts.instance.SendTimedMetrics()
			wc.Done()
		}()
	}

	wc.Wait()

	ts.instance.Add(nil, []byte("path1"), []byte("key1"), []byte("value1"), defaultTTL)
	ts.instance.SendTimedMetrics()

	ts.Equal(2, ts.metrics.numCommandExecution, "expects two command execution events")
	ts.Equal(2, ts.metrics.numCommandExecutionElapsedTime, "expects tow command execution time measurements")
	ts.Equal(3, ts.metrics.numResourcesChangeEvent, "expected three event changing the number of resources")
	ts.Equal(1, ts.metrics.numNoResourcesAvailableEvents, "expected one no resource available events")
	ts.Equal(1, ts.metrics.numAvailableResourcesRestoredEvent, "expected one resource available events")
	ts.Equal(0, ts.telnetMetrics.numResolveAddressElapsedTime, "expected a resolve address event")
	ts.Equal(0, ts.telnetMetrics.numDialElapsedTime, "expected a dial event")
	ts.Equal(2, ts.telnetMetrics.numSendElapsedTime, "expected two send event")
	ts.Equal(2, ts.telnetMetrics.numWriteElapsedTime, "expected two write event")
	ts.Equal(2, ts.telnetMetrics.numReadElapsedTime, "expected two read event")
	ts.Equal(0, ts.telnetMetrics.numCloseElapsedTime, "expected a close event")
}

func TestZencachedNoResourceMetricsSuite(t *testing.T) {

	suite.Run(t,
		&zencachedNoResourceMetricsTestSuite{
			zencachedMetricsTestSuite: zencachedMetricsTestSuite{
				commandExecutionBufferSize: 1,
			},
		},
	)
}
