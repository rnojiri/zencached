package zencached_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/rnojiri/dockerh"
	"github.com/rnojiri/logh"
	"github.com/rnojiri/zencached"
)

const numNodes int = 3

var (
	memcachedPodNames []string
	memcachedPodPort  []int
)

func init() {

	memcachedPodNames = make([]string, numNodes)
	memcachedPodPort = make([]int, numNodes)

	for i := 0; i < numNodes; i++ {
		memcachedPodNames[i] = fmt.Sprintf("memcached-pod%d", i)
		memcachedPodPort[i] = 11211 + i
	}
}

func randomResultType() zencached.ResultType {
	return zencached.ResultTypeValues()[rand.Int()%len(zencached.ResultTypeValues())]
}

func createExtraMemcachedPod(t *testing.T) (newPodName string, newNode zencached.Node) {

	var err error
	newPodName = "extra-memcached-pod"
	newNode.Port = memcachedPodPort[len(memcachedPodPort)-1] + 1
	newNode.Host, err = dockerh.CreateMemcached(newPodName, newNode.Port, 64)
	assert.NoError(t, err, "expected no error creating a new pod")

	return
}

//
// Telnet tests.
// author: rnojiri
//

// startMemcachedCluster - setup the nodes and returns the addresses
func startMemcachedCluster() []zencached.Node {

	wc := sync.WaitGroup{}
	wc.Add(numNodes)

	var err error
	nodes := make([]zencached.Node, numNodes)

	for i := 0; i < numNodes; i++ {

		go func(i int) {
			nodes[i].Host, err = dockerh.CreateMemcached(memcachedPodNames[i], memcachedPodPort[i], 64)
			if err != nil {
				panic(err)
			}

			nodes[i].Port = memcachedPodPort[i]

			wc.Done()
		}(i)
	}

	wc.Wait()

	return nodes
}

func terminatePods() {

	for i := 0; i < numNodes; i++ {
		dockerh.Remove(memcachedPodNames[i])
	}
}

type telnetMetricsCollector struct {
	numResolveAddressElapsedTime int
	numDialElapsedTime           int
	numCloseElapsedTime          int
	numSendElapsedTime           int
	numWriteElapsedTime          int
	numReadElapsedTime           int
}

func (tmc *telnetMetricsCollector) ResolveAddressElapsedTime(node string, elapsedTime int64) {

	tmc.numResolveAddressElapsedTime++
}

func (tmc *telnetMetricsCollector) DialElapsedTime(node string, elapsedTime int64) {

	tmc.numDialElapsedTime++
}

func (tmc *telnetMetricsCollector) CloseElapsedTime(node string, elapsedTime int64) {

	tmc.numCloseElapsedTime++
}

func (tmc *telnetMetricsCollector) SendElapsedTime(node string, elapsedTime int64) {

	tmc.numSendElapsedTime++
}

func (tmc *telnetMetricsCollector) WriteElapsedTime(node string, elapsedTime int64) {

	tmc.numWriteElapsedTime++
}

func (tmc *telnetMetricsCollector) ReadElapsedTime(node string, elapsedTime int64) {

	tmc.numReadElapsedTime++
}

func (tmc *telnetMetricsCollector) reset() {

	tmc.numResolveAddressElapsedTime = 0
	tmc.numDialElapsedTime = 0
	tmc.numCloseElapsedTime = 0
	tmc.numSendElapsedTime = 0
	tmc.numWriteElapsedTime = 0
	tmc.numReadElapsedTime = 0
}

type telnetTestSuite struct {
	suite.Suite
	telnet           *zencached.Telnet
	enableMetrics    bool
	metricsCollector *telnetMetricsCollector
}

// createTelnetConf - creates a new telnet configuration
func createTelnetConf(metricsCollector zencached.TelnetMetricsCollector) *zencached.TelnetConfiguration {

	tc := &zencached.TelnetConfiguration{
		ReconnectionTimeout:    time.Second,
		MaxWriteTimeout:        time.Second,
		MaxReadTimeout:         time.Second,
		HostConnectionTimeout:  time.Second,
		MaxWriteRetries:        3,
		ReadBufferSize:         2048,
		TelnetMetricsCollector: metricsCollector,
	}

	return tc
}

func (ts *telnetTestSuite) SetupSuite() {

	logh.ConfigureGlobalLogger(logh.DEBUG, logh.CONSOLE)

	terminatePods()

	nodes := startMemcachedCluster()

	if ts.enableMetrics {
		ts.metricsCollector = &telnetMetricsCollector{}
	}

	var err error
	ts.telnet, err = zencached.NewTelnet(nodes[rand.Intn(len(nodes))], *createTelnetConf(ts.metricsCollector))
	if err != nil {
		ts.T().Fatal(err)
	}
}

func (ts *telnetTestSuite) TearDownTest() {

	if ts.enableMetrics {
		ts.metricsCollector.reset()
	}
}

func (ts *telnetTestSuite) TearDownSuite() {

	ts.telnet.Close()

	terminatePods()
}

// TestConnectionOpenClose - tests the open and close
func (ts *telnetTestSuite) TestConnectionOpenClose() {

	err := ts.telnet.Connect()
	if !ts.NoError(err, "error connecting") {
		return
	}

	ts.telnet.Close()

	if ts.enableMetrics {
		ts.Equal(1, ts.metricsCollector.numResolveAddressElapsedTime, "expected a resolve address event")
		ts.Equal(1, ts.metricsCollector.numDialElapsedTime, "expected a dial event")
		ts.Equal(1, ts.metricsCollector.numCloseElapsedTime, "expected a close event")
	}
}

// TestInfoCommand - tests a simple info command
func (ts *telnetTestSuite) TestInfoCommand() {

	err := ts.telnet.Connect()
	if !ts.NoError(err, "error connecting") {
		return
	}

	err = ts.telnet.Send([]byte("stats\r\n"))
	if !ts.NoError(err, "error sending command") {
		return
	}

	expectedRandomType := randomResultType()

	payload, resultType, err := ts.telnet.Read(
		zencached.TelnetResponseSet{
			ResponseSets: [][]byte{[]byte("END")},
			ResultTypes:  []zencached.ResultType{expectedRandomType},
		},
	)
	if !ts.NoError(err, "error reading response") {
		return
	}

	ts.Equal(expectedRandomType, resultType, "expected same result type")
	ts.True(regexp.MustCompile(`STAT version [0-9\\.]+`).MatchString(string(payload)), "version not found")

	ts.telnet.Close()

	if ts.enableMetrics {
		ts.Equal(1, ts.metricsCollector.numResolveAddressElapsedTime, "expected a resolve address event")
		ts.Equal(1, ts.metricsCollector.numDialElapsedTime, "expected a dial event")
		ts.Equal(1, ts.metricsCollector.numSendElapsedTime, "expected a send event")
		ts.Equal(1, ts.metricsCollector.numWriteElapsedTime, "expected a write event")
		ts.Equal(1, ts.metricsCollector.numReadElapsedTime, "expected a read event")
		ts.Equal(1, ts.metricsCollector.numCloseElapsedTime, "expected a close event")
	}
}

// TestInsertCommand - tests a simple insert command
func (ts *telnetTestSuite) TestInsertCommand() {

	err := ts.telnet.Connect()
	if !ts.NoError(err, "error connecting") {
		return
	}

	err = ts.telnet.Send([]byte("add gotest 0 10 4\r\ntest\r\n"))
	if !ts.NoError(err, "error sending set command") {
		return
	}

	expectedRandomType := randomResultType()

	payload, resultType, err := ts.telnet.Read(
		zencached.TelnetResponseSet{
			ResponseSets: [][]byte{[]byte("STORED")},
			ResultTypes:  []zencached.ResultType{expectedRandomType},
		},
	)
	if !ts.NoError(err, "error reading response") {
		return
	}

	ts.Equal(expectedRandomType, resultType, "expected same result type")
	ts.True(bytes.Contains(payload, []byte("STORED")), "expected \"STORED\" as answer")

	err = ts.telnet.Send([]byte("get gotest\r\n"))
	if !ts.NoError(err, "error sending get command") {
		return
	}

	expectedRandomType = randomResultType()

	payload, resultType, err = ts.telnet.Read(
		zencached.TelnetResponseSet{
			ResponseSets: [][]byte{[]byte("END")},
			ResultTypes:  []zencached.ResultType{expectedRandomType},
		},
	)
	if !ts.NoError(err, "error reading response") {
		return
	}

	ts.Equal(expectedRandomType, resultType, "expected same result type")
	ts.True(bytes.Contains(payload, []byte("test")), "expected \"test\" to be stored")

	ts.telnet.Close()

	if ts.enableMetrics {
		ts.Equal(1, ts.metricsCollector.numResolveAddressElapsedTime, "expected a resolve address event")
		ts.Equal(1, ts.metricsCollector.numDialElapsedTime, "expected a dial event")
		ts.Equal(2, ts.metricsCollector.numSendElapsedTime, "expected a send event")
		ts.Equal(2, ts.metricsCollector.numWriteElapsedTime, "expected a write event")
		ts.Equal(2, ts.metricsCollector.numReadElapsedTime, "expected a read event")
		ts.Equal(1, ts.metricsCollector.numCloseElapsedTime, "expected a close event")
	}
}

func TestTelnetSuite(t *testing.T) {

	suite.Run(t,
		&telnetTestSuite{
			enableMetrics: false,
		},
	)
}

func TestTelnetWithMetricsSuite(t *testing.T) {

	suite.Run(t,
		&telnetTestSuite{
			enableMetrics: true,
		},
	)
}
