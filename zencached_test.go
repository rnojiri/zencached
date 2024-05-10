package zencached_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rnojiri/logh"
	"github.com/rnojiri/zencached"
	"github.com/stretchr/testify/suite"
)

// Tests for zencached basic operations
// author: rnojiri

var defaultTTL uint64 = 60

type zencachedTestSuite struct {
	suite.Suite
	instance *zencached.Zencached
	config   *zencached.Configuration
}

func (ts *zencachedTestSuite) SetupSuite() {

	logh.ConfigureGlobalLogger(logh.DEBUG, logh.CONSOLE)

	terminatePods()

	nodes := startMemcachedCluster()

	var err error
	ts.instance, ts.config, err = createZencached(nodes, false, nil, nil)
	if err != nil {
		ts.T().Fatalf("expected no errors creating zencached: %v", err)
	}
}

func (ts *zencachedTestSuite) TearDownSuite() {

	ts.instance.Shutdown()

	terminatePods()
}

// createZencached - creates a new client
func createZencached(nodes []zencached.Node, rebalanceOnDisconnection bool, metricCollector zencached.ZencachedMetricsCollector, telnetMetricsCollector zencached.TelnetMetricsCollector) (*zencached.Zencached, *zencached.Configuration, error) {

	c := &zencached.Configuration{
		Nodes:                     nodes,
		NumConnectionsPerNode:     2,
		TelnetConfiguration:       *createTelnetConf(telnetMetricsCollector),
		ZencachedMetricsCollector: metricCollector,
		RebalanceOnDisconnection:  rebalanceOnDisconnection,
		NumNodeListRetries:        1,
		NodeListRetryTimeout:      100 * time.Millisecond,
	}

	z, err := zencached.New(c)
	if err != nil {
		return nil, nil, err
	}

	return z, c, nil
}

// TestRouting - tests the routing algorithm
func (ts *zencachedTestSuite) TestRouting() {

	f := func(path, key []byte, expected int) bool {

		_, index, err := ts.instance.GetConnectedNodeWorkers(key, path, key)
		if !ts.NoError(err, "expects no error getting a connection") {
			return false
		}

		if !ts.Equalf(expected, index, "expected index %d", expected) {
			return false
		}

		return true
	}

	if !f([]byte{0, 1}, []byte{2, 255}, 0) { //should be index 0
		return
	}

	if !f([]byte{10, 199}, []byte{202, 149}, 2) { //should be index 2
		return
	}

	if !f([]byte{206, 98}, []byte{60, 4}, 1) { //should be index 1
		return
	}

	if !f([]byte{206, 98}, []byte{60, 3}, 0) { //should be index 0
		return
	}
}

// TestAddCommand - tests the add command
func (ts *zencachedTestSuite) TestAddCommand() {

	f := func(route []byte, path, key, value string, expectedStored bool, testIndex int) {

		stored, err := ts.instance.Add(route, []byte(path), []byte(key), []byte(value), defaultTTL)
		if !ts.NoError(err, "expected no error storing key") {
			return
		}

		ts.Truef(expectedStored == stored, "unexpected storage status for test %d and key %s", testIndex, key)
	}

	f([]byte{3}, "path", "test1", "test1", true, 1)
	f([]byte{3}, "path", "test2", "test2", true, 1)
	f([]byte{9}, "path", "test1", "error", false, 1)

	f([]byte{4}, "path", "test1", "test1", true, 2)
	f([]byte{4}, "path", "test2", "test2", true, 2)
	f([]byte{7}, "path", "test1", "error", false, 2)

	f([]byte{5}, "path", "test1", "test1", true, 3)
	f([]byte{5}, "path", "test2", "test2", true, 3)
	f([]byte{8}, "path", "test1", "error", false, 3)
}

// rawSetKey - sets a key on memcached using raw command
func (ts *zencachedTestSuite) rawSetKey(telnetConn *zencached.Telnet, path, key, value string) {

	err := telnetConn.Send([]byte(fmt.Sprintf("set %s%s 0 %d %d\r\n%s\r\n", path, key, 60, len(value), value)))
	if !ts.NoError(err, "expected no error sending set command") {
		return
	}

	_, err = telnetConn.Read([][]byte{[]byte("STORED")})
	if !ts.NoError(err, "expected no error reading key") {
		return
	}
}

// TestGetCommand - tests the get command
func (ts *zencachedTestSuite) TestGetCommand() {

	f := func(route []byte, path, key, value string, testIndex int) {

		nw, _, err := ts.instance.GetConnectedNodeWorkers(route, []byte(path), []byte(key))
		if !ts.NoError(err, "expects no error getting a connection") {
			return
		}

		t, err := nw.NewTelnetFromNode()
		if !ts.NoError(err, "expected no error creating telnet connection") {
			return
		}

		defer t.Close()

		ts.rawSetKey(t, path, key, value)

		response, found, err := ts.instance.Get(route, []byte(path), []byte(key))
		if !ts.NoError(err, "expected no error storing key") {
			return
		}

		if !ts.Truef(found, "expected value from key \"%s\" to be found on test %d", key, testIndex) {
			return
		}

		ts.Equal([]byte(value), response, "expected values to be equal")
	}

	f([]byte{3}, "path", "test1", "test1", 1)
	f([]byte{3}, "path", "test2", "test2", 2)
	f([]byte{9}, "path", "test1", "test3", 3)
	f([]byte{4}, "path", "test4", "test4", 4)
	f([]byte{4}, "path", "test5", "test5", 5)
	f([]byte{7}, "path", "test5", "test6", 6)
	f([]byte{5}, "path", "test7", "test7", 7)
	f([]byte{5}, "path", "test8", "test8", 8)
	f([]byte{8}, "path", "test7", "test8", 9)
}

// TestSetCommand - tests the set command
func (ts *zencachedTestSuite) TestSetCommand() {

	f := func(route []byte, path, key, value string, testIndex int) {

		stored, err := ts.instance.Set(route, []byte(path), []byte(key), []byte(value), defaultTTL)
		if !ts.NoError(err, "expected no error storing key") {
			return
		}

		if !ts.Truef(stored, "unexpected storage status for test %d", testIndex, key) {
			return
		}

		storedValue, found, err := ts.instance.Get(route, []byte(path), []byte(key))
		if !ts.NoError(err, "expected no error getting key") {
			return
		}

		if !ts.Truef(found, "unexpected get status for test %d", testIndex, key) {
			return
		}

		ts.Equal([]byte(value), storedValue, "expected the same values")
	}

	f([]byte{3}, "path", "test1", "test1", 1)
	f([]byte{3}, "path", "test2", "test2", 2)
	f([]byte{9}, "path", "test1", "test3", 3)
	f([]byte{4}, "path", "test4", "test4", 4)
	f([]byte{4}, "path", "test5", "test5", 5)
	f([]byte{7}, "path", "test5", "test6", 6)
	f([]byte{5}, "path", "test7", "test7", 7)
	f([]byte{5}, "path", "test8", "test8", 8)
	f([]byte{8}, "path", "test7", "test8", 9)
}

// TestDeleteCommand - tests the delete command
func (ts *zencachedTestSuite) TestDeleteCommand() {

	f := func(route []byte, path, key, value string, setValue bool, testIndex int) {

		nw, _, err := ts.instance.GetConnectedNodeWorkers(route, []byte(path), []byte(key))
		if !ts.NoError(err, "expects no error getting a connection") {
			return
		}

		t, err := nw.NewTelnetFromNode()
		if !ts.NoError(err, "expected no error creating telnet connection") {
			return
		}

		defer t.Close()

		if setValue {
			ts.rawSetKey(t, path, key, value)
		}

		status, err := ts.instance.Delete(route, []byte(path), []byte(key))
		if !ts.NoError(err, "expected no error storing key") {
			return
		}

		if !ts.Truef(status == setValue, "unexpected delete status for test %d", testIndex, key) {
			return
		}

		if setValue {
			_, found, err := ts.instance.Get(route, []byte(path), []byte(key))
			if !ts.NoError(err, "expected no error storing key") {
				return
			}

			if !ts.Truef(!found, "unexpected get status for test %d", testIndex, key) {
				return
			}
		}
	}

	f([]byte{3}, "path", "test1", "test1", true, 1)
	f([]byte{3}, "path", "test2", "test2", true, 2)
	f([]byte{9}, "path", "test1", "test3", false, 3)
	f([]byte{4}, "path", "test4", "test4", true, 4)
	f([]byte{4}, "path", "test5", "test5", true, 5)
	f([]byte{7}, "path", "test5", "test6", false, 6)
	f([]byte{5}, "path", "test7", "test7", true, 7)
	f([]byte{5}, "path", "test8", "test8", true, 8)
	f([]byte{8}, "path", "test7", "test8", false, 9)
}

// TestVersionCommand - tests the version command
func (ts *zencachedTestSuite) TestVersionCommand() {

	nw, _, err := ts.instance.GetConnectedNodeWorkers(nil, nil, nil) //any
	if !ts.NoError(err, "expects no error getting a connection") {
		return
	}

	t, err := nw.NewTelnetFromNode()
	if !ts.NoError(err, "expected no error creating telnet connection") {
		return
	}

	defer t.Close()

	err = t.Send([]byte("version\r\n"))
	if !ts.NoError(err, "error sending version command") {
		return
	}

	payload, err := t.Read([][]byte{[]byte("VERSION")})
	if !ts.NoError(err, "error reading response") {
		return
	}

	value, err := ts.instance.Version(nil)
	if !ts.NoError(err, "error getting version") {
		return
	}

	ts.Equal(strings.Split(strings.TrimSpace(string(payload)), " ")[1], string(value), "expected same version")
}

func TestZencachedSuite(t *testing.T) {

	suite.Run(t, new(zencachedTestSuite))
}
