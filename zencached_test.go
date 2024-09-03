package zencached_test

import (
	"bytes"
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
	instance           *zencached.Zencached
	config             *zencached.Configuration
	compressionType    zencached.CompressionType
	compressorInstance zencached.DataCompressor
}

func (ts *zencachedTestSuite) SetupSuite() {

	logh.ConfigureGlobalLogger(logh.DEBUG, logh.CONSOLE)

	terminatePods()

	nodes := startMemcachedCluster()

	var err error

	switch ts.compressionType {
	case zencached.CompressionTypeBase64:
		ts.compressorInstance, err = zencached.NewDataCompressor(zencached.CompressionTypeBase64, 0)
	case zencached.CompressionTypeZstandard:
		ts.compressorInstance, err = zencached.NewDataCompressor(zencached.CompressionTypeZstandard, 5)
	}

	ts.instance, ts.config, err = createZencached(nodes, 10, false, nil, nil, ts.compressionType)
	if err != nil {
		ts.T().Fatalf("expected no errors creating zencached: %v", err)
	}
}

func (ts *zencachedTestSuite) TearDownSuite() {

	ts.instance.Shutdown()

	terminatePods()
}

// createZencached - creates a new client
func createZencached(
	nodes []zencached.Node,
	commandExecutionBufferSize uint32,
	rebalanceOnDisconnection bool,
	metricCollector zencached.ZencachedMetricsCollector,
	telnetMetricsCollector zencached.TelnetMetricsCollector,
	compressionType zencached.CompressionType,
) (*zencached.Zencached, *zencached.Configuration, error) {

	c := &zencached.Configuration{
		Nodes:                      nodes,
		NumConnectionsPerNode:      3,
		TelnetConfiguration:        *createTelnetConf(telnetMetricsCollector),
		ZencachedMetricsCollector:  metricCollector,
		RebalanceOnDisconnection:   rebalanceOnDisconnection,
		NumNodeListRetries:         3,
		NodeListRetryTimeout:       2 * time.Second,
		CommandExecutionBufferSize: commandExecutionBufferSize,
		DisableTimedMetrics:        true, // using this to disable the automatic metrics
		CompressionConfiguration: zencached.CompressionConfiguration{
			CompressionType:  compressionType,
			CompressionLevel: 5,
		},
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

func (ts *zencachedTestSuite) genericInvalidKeyTests(opfuncWrapper func(route, path, key []byte) []error) {

	genericTests := func(path, key []byte, keyIssue string) {

		route := []byte("route")

		errs := opfuncWrapper(route, path, key)
		for _, err := range errs {
			ts.ErrorContainsf(err, zencached.ErrInvalidKeyFormat.Error(), "expected invalid key format providing all parameters: %s", keyIssue)
		}

		errs = opfuncWrapper(nil, path, key)
		for _, err := range errs {
			ts.ErrorContainsf(err, zencached.ErrInvalidKeyFormat.Error(), "expected invalid key format not providing route: %s", keyIssue)
		}

		errs = opfuncWrapper(nil, nil, key)
		for _, err := range errs {
			ts.ErrorContainsf(err, zencached.ErrInvalidKeyFormat.Error(), "expected invalid key format not providing route and path: %s", keyIssue)
		}

		errs = opfuncWrapper(nil, nil, nil)
		for _, err := range errs {
			ts.ErrorContainsf(err, zencached.ErrInvalidKeyFormat.Error(), "expected invalid key format not providing anything: %s", keyIssue)
		}
	}

	path := []byte("pa th")
	key := []byte("ke y")

	genericTests(path, key, "key with spaces")

	path = []byte("pa\nth")
	key = []byte("ke\ny")

	genericTests(path, key, "key with new lines")

	path = []byte("")
	key = []byte("")

	genericTests(path, key, "key with empty path and key")

	path = []byte("Bxyf7xtxtg4byuQwhnkATRQiq4R1wW37a5rjMrTL42Zf0r9F1SzBFt89nAy9zxPYitwLbmeEaPaA6pt5cBha0JJmbu61hHyPCP2vv21Eq7Net0qJdETaxiKtpxwAcZTfFWdy1QhfiFfPnXyW9QxeqZtEWVtbgBWFDJ1cRFTvDNA3qNrvp8vn4BbFbcJnzATLadARxDpBYe0CgjyaNKS3c5U7kGwdEkxMCh3jGJviyjagwhwB8bmk20YFZcZ")
	key = []byte("Bxyf7xtxtg4byuQwhnkATRQiq4R1wW37a5rjMrTL42Zf0r9F1SzBFt89nAy9zxPYitwLbmeEaPaA6pt5cBha0JJmbu61hHyPCP2vv21Eq7Net0qJdETaxiKtpxwAcZTfFWdy1QhfiFfPnXyW9QxeqZtEWVtbgBWFDJ1cRFTvDNA3qNrvp8vn4BbFbcJnzATLadARxDpBYe0CgjyaNKS3c5U7kGwdEkxMCh3jGJviyjagwhwB8bmk20YFZcZ")

	genericTests(path, key, "key with more than 250 characters")
}

// TestAddCommandWithInvalidKey - tests the add command with invalid keys
func (ts *zencachedTestSuite) TestAddCommandWithInvalidKey() {

	ts.genericInvalidKeyTests(
		func(route, path, key []byte) []error {
			value := []byte("value")
			_, err := ts.instance.Add(route, path, key, value, defaultTTL)
			return []error{err}
		},
	)
}

// TestSetCommandWithInvalidKey - tests the set command with invalid keys
func (ts *zencachedTestSuite) TestSetCommandWithInvalidKey() {

	ts.genericInvalidKeyTests(
		func(route, path, key []byte) []error {
			value := []byte("value")
			_, err := ts.instance.Set(route, path, key, value, defaultTTL)
			return []error{err}
		},
	)
}

// TestGetCommandWithInvalidKey - tests the get command with invalid keys
func (ts *zencachedTestSuite) TestGetCommandWithInvalidKey() {

	ts.genericInvalidKeyTests(
		func(route, path, key []byte) []error {
			_, err := ts.instance.Get(route, path, key)
			return []error{err}
		},
	)
}

// TestDeleteCommandWithInvalidKey - tests the delete command with invalid keys
func (ts *zencachedTestSuite) TestDeleteCommandWithInvalidKey() {

	ts.genericInvalidKeyTests(
		func(route, path, key []byte) []error {
			_, err := ts.instance.Delete(route, path, key)
			return []error{err}
		},
	)
}

// TestAddCommand - tests the add command
func (ts *zencachedTestSuite) TestAddCommand() {

	f := func(route []byte, path, key, value string, expectedResultType zencached.ResultType, testIndex int) {

		result, err := ts.instance.Add(route, []byte(path), []byte(key), []byte(value), defaultTTL)
		if !ts.NoError(err, "expected no error storing key") {
			return
		}

		ts.Truef(expectedResultType == result.Type, "unexpected storage status for test %d and key %s: received %s, expected %s", testIndex, key, result.Type.String(), expectedResultType.String())
	}

	f([]byte{3}, "path", "test1", "test1", zencached.ResultTypeStored, 1)
	f([]byte{3}, "path", "test2", "test2", zencached.ResultTypeStored, 1)
	f([]byte{9}, "path", "test1", "error", zencached.ResultTypeNotStored, 1)

	f([]byte{4}, "path", "test1", "test1", zencached.ResultTypeStored, 2)
	f([]byte{4}, "path", "test2", "test2", zencached.ResultTypeStored, 2)
	f([]byte{7}, "path", "test1", "error", zencached.ResultTypeNotStored, 2)

	f([]byte{5}, "path", "test1", "test1", zencached.ResultTypeStored, 3)
	f([]byte{5}, "path", "test2", "test2", zencached.ResultTypeStored, 3)
	f([]byte{8}, "path", "test1", "error", zencached.ResultTypeNotStored, 3)
}

// rawSetKey - sets a key on memcached using raw command
func (ts *zencachedTestSuite) rawSetKey(telnetConn *zencached.Telnet, path, key, value string) {

	// must compress or encode if the configuration is enabled before set the value

	var err error
	tvalue := []byte(value)

	if ts.compressionType != zencached.CompressionTypeNone {

		tvalue, err = ts.compressorInstance.Compress(tvalue)
		if !ts.NoError(err, "expected no error compressing value") {
			return
		}

	}

	err = telnetConn.Send([]byte(fmt.Sprintf("set %s%s 0 %d %d\r\n%s\r\n", path, key, 60, len(tvalue), string(tvalue))))
	if !ts.NoError(err, "expected no error sending set command") {
		return
	}

	expectedRandomType := randomResultType()

	_, resultType, err := telnetConn.Read(
		zencached.TelnetResponseSet{
			ResponseSets: [][]byte{[]byte("STORED")},
			ResultTypes:  []zencached.ResultType{expectedRandomType},
		},
	)
	if !ts.NoError(err, "expected no error reading key") {
		return
	}

	ts.Equal(expectedRandomType, resultType, "expected same result type")
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

		result, err := ts.instance.Get(route, []byte(path), []byte(key))
		if !ts.NoError(err, "expected no error retrieving key") {
			return
		}

		if !ts.Equal(zencached.ResultTypeFound, result.Type, "expected value from key \"%s\" to be found on test %d", key, testIndex) {
			return
		}

		ts.Equal([]byte(value), result.Data, "expected values to be equal")
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

		result, err := ts.instance.Set(route, []byte(path), []byte(key), []byte(value), defaultTTL)
		if !ts.NoError(err, "expected no error storing key") {
			return
		}

		if !ts.Equal(zencached.ResultTypeStored, result.Type, "unexpected storage status for test %d", testIndex, key) {
			return
		}

		result, err = ts.instance.Get(route, []byte(path), []byte(key))
		if !ts.NoError(err, "expected no error getting key") {
			return
		}

		if !ts.Equal(zencached.ResultTypeFound, result.Type, "unexpected get status for test %d", testIndex, key) {
			return
		}

		ts.Equal([]byte(value), result.Data, "expected the same values")
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

		result, err := ts.instance.Delete(route, []byte(path), []byte(key))
		if !ts.NoError(err, "expected no error storing key") {
			return
		}

		if setValue {
			if !ts.Equal(zencached.ResultTypeDeleted, result.Type, "unexpected delete status for test %d", testIndex, key) {
				return
			}

			result, err := ts.instance.Get(route, []byte(path), []byte(key))
			if !ts.NoError(err, "expected no error storing key") {
				return
			}

			if !ts.Equal(zencached.ResultTypeNotFound, result.Type, "unexpected get status for test %d", testIndex, key) {
				return
			}

		} else {

			if !ts.Equal(zencached.ResultTypeNotFound, result.Type, "unexpected delete status for test %d", testIndex, key) {
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

	expectedRandomType := randomResultType()

	payload, resultType, err := t.Read(
		zencached.TelnetResponseSet{
			ResponseSets: [][]byte{[]byte("VERSION")},
			ResultTypes:  []zencached.ResultType{expectedRandomType},
		},
	)
	if !ts.NoError(err, "error reading response") {
		return
	}

	ts.Equal(expectedRandomType, resultType, "expected same result type")

	result, err := ts.instance.Version(nil)
	if !ts.NoError(err, "error getting version") {
		return
	}

	ts.Equal(strings.Split(strings.TrimSpace(string(payload)), " ")[1], string(result.Data), "expected same version")
}

func (ts *zencachedTestSuite) extractValueFromRawTelnet(response []byte) []byte {

	extractedArray := bytes.Split(response, []byte{'\r', '\n'})
	if len(extractedArray) != 4 {
		ts.Fail("expected 4 lines of response")
		return nil
	}

	var err error
	extractedValue := extractedArray[1]

	if ts.compressionType != zencached.CompressionTypeNone {
		extractedValue, err = ts.compressorInstance.Decompress(extractedValue)
		if !ts.NoError(err, "expected success decompressing data") {
			return nil
		}
	}

	return extractedValue
}

func TestZencachedSuite(t *testing.T) {

	suite.Run(t, new(zencachedTestSuite))
}

func TestBase64ZencachedSuite(t *testing.T) {

	suite.Run(t,
		&zencachedTestSuite{
			compressionType: zencached.CompressionTypeBase64,
		},
	)
}
func TestZstandardZencachedSuite(t *testing.T) {

	suite.Run(t,
		&zencachedTestSuite{
			compressionType: zencached.CompressionTypeZstandard,
		},
	)
}
