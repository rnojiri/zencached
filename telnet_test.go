package zencached_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"regexp"
	"sync"
	"testing"
	"time"

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

type telnetTestSuite struct {
	suite.Suite
	telnet *zencached.Telnet
}

// createTelnetConf - creates a new telnet configuration
func createTelnetConf() *zencached.TelnetConfiguration {

	return &zencached.TelnetConfiguration{
		ReconnectionTimeout: 1 * time.Second,
		MaxWriteTimeout:     5 * time.Second,
		MaxReadTimeout:      5 * time.Second,
		MaxWriteRetries:     3,
		ReadBufferSize:      2048,
	}
}

func (ts *telnetTestSuite) SetupSuite() {

	logh.ConfigureGlobalLogger(logh.DEBUG, logh.CONSOLE)

	terminatePods()

	nodes := startMemcachedCluster()

	var err error
	ts.telnet, err = zencached.NewTelnet(&nodes[rand.Intn(len(nodes))], createTelnetConf())
	if err != nil {
		ts.T().Fatal(err)
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
}

// TestInfoCommand - tests a simple info command
func (ts *telnetTestSuite) TestInfoCommand() {

	err := ts.telnet.Connect()
	if !ts.NoError(err, "error connecting") {
		return
	}

	defer ts.telnet.Close()

	err = ts.telnet.Send([]byte("stats\r\n"))
	if !ts.NoError(err, "error sending command") {
		return
	}

	payload, err := ts.telnet.Read([][]byte{[]byte("END")})
	if !ts.NoError(err, "error reading response") {
		return
	}

	ts.True(regexp.MustCompile(`STAT version [0-9\\.]+`).MatchString(string(payload)), "version not found")
}

// TestInsertCommand - tests a simple insert command
func (ts *telnetTestSuite) TestInsertCommand() {

	err := ts.telnet.Connect()
	if !ts.NoError(err, "error connecting") {
		return
	}

	defer ts.telnet.Close()

	err = ts.telnet.Send([]byte("add gotest 0 10 4\r\ntest\r\n"))
	if !ts.NoError(err, "error sending set command") {
		return
	}

	payload, err := ts.telnet.Read([][]byte{[]byte("STORED")})
	if !ts.NoError(err, "error reading response") {
		return
	}

	ts.True(bytes.Contains(payload, []byte("STORED")), "expected \"STORED\" as answer")

	err = ts.telnet.Send([]byte("get gotest\r\n"))
	if !ts.NoError(err, "error sending get command") {
		return
	}

	payload, err = ts.telnet.Read([][]byte{[]byte("END")})
	if !ts.NoError(err, "error reading response") {
		return
	}

	ts.True(bytes.Contains(payload, []byte("test")), "expected \"test\" to be stored")
}

func TestTelnetSuite(t *testing.T) {

	suite.Run(t, new(telnetTestSuite))
}
