package zencached_test

import (
	"bytes"
)

// TestClusterSetCommand - tests the cluster storage command
func (ts *zencachedTestSuite) TestClusterSetCommand() {

	path := []byte("path")
	key := []byte("cluster-storage")
	value := []byte("cluster-value-storage")

	stored, errors := ts.instance.ClusterSet(path, key, value, defaultTTL)

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

		nw, err := ts.instance.GetNodeWorkersByIndex(i)
		if !ts.NoError(err, "expects no error getting a connection") {
			return
		}

		t, err := nw.NewTelnetFromNode()
		if !ts.NoError(err, "expected no error creating telnet connection") {
			return
		}

		defer t.Close()

		err = t.Send([]byte("get " + string(path) + string(key) + "\r\n"))
		if !ts.NoError(err, "expected success getting key") {
			return
		}

		response, err := t.Read([][]byte{[]byte("END")})
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
		nw, err := ts.instance.GetNodeWorkersByIndex(i)
		if !ts.NoError(err, "expects no error getting a connection") {
			return
		}

		t, err := nw.NewTelnetFromNode()
		if !ts.NoError(err, "expected no error creating telnet connection") {
			return
		}

		defer t.Close()

		ts.rawSetKey(t, path, key, value)
	}
}

// TestClusterGetCommand - tests the cluster get command
func (ts *zencachedTestSuite) TestClusterGetCommand() {

	path := "path"
	key := "cluster-get"
	value := "cluster-value-get"

	ts.rawSetKeyOnAllNodes(path, key, value)

	for i := 0; i < 1000; i++ {

		values, exists, errs := ts.instance.ClusterGet([]byte(path), []byte(key))

		for i := 0; i < numNodes; i++ {

			if !ts.NoErrorf(errs[i], "unexpected error on cluster get: %d", i) {
				return
			}

			if !ts.Truef(exists[i], "expected value to be stored on cluster get: %d", i) {
				return
			}

			if !ts.Equalf([]byte(value), values[i], "expected the same value stored", i) {
				return
			}
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

		nw, err := ts.instance.GetNodeWorkersByIndex(i)
		if !ts.NoError(err, "expects no error getting a connection") {
			return
		}

		t, err := nw.NewTelnetFromNode()
		if !ts.NoError(err, "expected no error creating telnet connection") {
			return
		}

		defer t.Close()

		err = t.Send([]byte("get " + key + "\r\n"))
		if !ts.NoError(err, "expected success getting key") {
			return
		}

		response, err := t.Read([][]byte{[]byte("END")})
		if !ts.NoError(err, "expected success reading last line") {
			return
		}

		if !ts.Truef(bytes.Contains(response, []byte("END")), "found a value stored on node: %d", i) {
			return
		}
	}
}
