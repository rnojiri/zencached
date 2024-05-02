package zencached_test

import (
	"bytes"

	"github.com/rnojiri/zencached"
)

// TestClusterStoreCommand - tests the cluster storage command
func (ts *zencachedTestSuite) TestClusterStoreCommand() {

	path := []byte("path")
	key := []byte("cluster-storage")
	value := []byte("cluster-value-storage")

	stored, errors := ts.instance.ClusterStore(zencached.Set, path, key, value, defaultTTL)

	if !ts.Len(stored, numNodes, "wrong number of nodes") {
		return
	}

	for i := 0; i < numNodes; i++ {
		if !ts.NoErrorf(errors[i], "unexpected error on node: %d", i) {
			return
		}

		if !ts.Truef(stored[i], "expected storage on node: %d", i) {
			return
		}

		telnetConn, err := ts.instance.GetTelnetConnByNodeIndex(i)
		if !ts.NoError(err, "expects no error getting a connection") {
			return
		}

		defer ts.instance.ReturnTelnetConnection(telnetConn, i)

		err = telnetConn.Send([]byte("get " + string(path) + string(key) + "\r\n"))
		if !ts.NoError(err, "expected success getting key") {
			return
		}

		response, err := telnetConn.Read([][]byte{[]byte("END")})
		if !ts.NoError(err, "expected success reading last line") {
			return
		}

		if !ts.Truef(bytes.Contains(response, value), "expected value to be stored on node: %d", i) {
			return
		}
	}
}

// rawSetKeyOnAllNodes - set the key and value on all nodes
func (ts *zencachedTestSuite) rawSetKeyOnAllNodes(path, key, value string) {

	for i := 0; i < numNodes; i++ {
		telnetConn, err := ts.instance.GetTelnetConnByNodeIndex(i)
		if !ts.NoError(err, "expects no error getting a connection") {
			return
		}

		ts.rawSetKey(telnetConn, path, key, value)
		ts.instance.ReturnTelnetConnection(telnetConn, i)
	}
}

// TestClusterGetCommand - tests the cluster get command
func (ts *zencachedTestSuite) TestClusterGetCommand() {

	path := "path"
	key := "cluster-get"
	value := "cluster-value-get"

	ts.rawSetKeyOnAllNodes(path, key, value)

	for i := 0; i < 1000; i++ {

		storedValue, stored, err := ts.instance.ClusterGet([]byte(path), []byte(key))

		if !ts.NoErrorf(err, "unexpected error on tentative: %d", i) {
			return
		}

		if !ts.Truef(stored, "expected value to be stored on tentative: %d", i) {
			return
		}

		if !ts.Equalf([]byte(value), storedValue, "expected the same value stored", i) {
			return
		}
	}
}

// TestClusterDeleteCommand - tests the cluster delete command
func (ts *zencachedTestSuite) TestClusterDeleteCommand() {

	path := "path"
	key := "cluster-delete"
	value := "cluster-value-delete"

	ts.rawSetKeyOnAllNodes(path, key, value)

	stored, errors := ts.instance.ClusterDelete([]byte(path), []byte(key))

	if !ts.Len(stored, numNodes, "wrong number of nodes") {
		return
	}

	for i := 0; i < numNodes; i++ {

		if !ts.NoErrorf(errors[i], "unexpected error on node: %d", i) {
			return
		}

		if !ts.Truef(stored[i], "expected delete on node: %d", i) {
			return
		}

		telnetConn, err := ts.instance.GetTelnetConnByNodeIndex(i)
		if !ts.NoError(err, "expects no error getting a connection") {
			return
		}

		defer ts.instance.ReturnTelnetConnection(telnetConn, i)

		err = telnetConn.Send([]byte("get " + key + "\r\n"))
		if !ts.NoError(err, "expected success getting key") {
			return
		}

		response, err := telnetConn.Read([][]byte{[]byte("END")})
		if !ts.NoError(err, "expected success reading last line") {
			return
		}

		if !ts.Truef(bytes.Contains(response, []byte("END")), "found a value stored on node: %d", i) {
			return
		}
	}
}
