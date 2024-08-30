package zencached_test

import (
	"bytes"
	"fmt"

	"github.com/rnojiri/zencached"
)

// genericTestClusterStoreCommand - tests a generic store command
func (ts *zencachedTestSuite) genericTestClusterStoreCommand(command string, storageFunc func(path []byte, key []byte, value []byte, ttl uint64) ([]*zencached.OperationResult, []error)) {

	path := []byte("path")
	key := []byte(fmt.Sprintf("cluster-storage-%s", command))
	value := []byte(fmt.Sprintf("cluster-value-storage-%s", command))

	results, errors := storageFunc(path, key, value, defaultTTL)

	if !ts.Len(results, numNodes, "wrong number of nodes") {
		return
	}

	for i := 0; i < numNodes; i++ {

		if !ts.NoErrorf(errors[i], "unexpected error on node: %d", i) {
			return
		}

		if !ts.Equal(zencached.ResultTypeStored, results[i].Type, "expected storage on node: %d", i) {
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

		if !ts.Equal(t.GetNode(), results[i].Node, "expected same node") {
			return
		}

		defer t.Close()

		err = t.Send([]byte("get " + string(path) + string(key) + "\r\n"))
		if !ts.NoError(err, "expected success getting key") {
			return
		}

		response, _, err := t.Read(
			zencached.TelnetResponseSet{
				ResponseSets: [][]byte{[]byte("END")},
				ResultTypes:  []zencached.ResultType{zencached.ResultTypeNone},
			},
		)
		if !ts.NoError(err, "expected success reading last line") {
			return
		}

		if !ts.Truef(bytes.Contains(response, value), "expected value to be stored on node: %d", i) {
			return
		}
	}
}

// TestClusterAddCommand - tests the cluster add command
func (ts *zencachedTestSuite) TestClusterAddCommand() {

	ts.genericTestClusterStoreCommand("add", ts.instance.ClusterAdd)
}

// TestClusterSetCommand - tests the cluster set command
func (ts *zencachedTestSuite) TestClusterSetCommand() {

	ts.genericTestClusterStoreCommand("set", ts.instance.ClusterSet)
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

		results, errs := ts.instance.ClusterGet([]byte(path), []byte(key))

		for i := 0; i < numNodes; i++ {

			if !ts.NoErrorf(errs[i], "unexpected error on cluster get: %d", i) {
				return
			}

			if !ts.Equal(zencached.ResultTypeFound, results[i].Type, "expected value to be found on cluster get: %d", i) {
				return
			}

			if !ts.Equalf([]byte(value), results[i].Data, "expected the same value stored", i) {
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

	results, errors := ts.instance.ClusterDelete([]byte(path), []byte(key))

	if !ts.Len(results, numNodes, "wrong number of nodes") {
		return
	}

	for i := 0; i < numNodes; i++ {

		if !ts.NoErrorf(errors[i], "unexpected error on node: %d", i) {
			return
		}

		if !ts.Equal(zencached.ResultTypeDeleted, results[i].Type, "expected delete on node: %d", i) {
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

		if !ts.Equal(t.GetNode(), results[i].Node, "expected same node") {
			return
		}

		defer t.Close()

		err = t.Send([]byte("get " + key + "\r\n"))
		if !ts.NoError(err, "expected success getting key") {
			return
		}

		response, _, err := t.Read(
			zencached.TelnetResponseSet{
				ResponseSets: [][]byte{[]byte("END")},
				ResultTypes:  []zencached.ResultType{zencached.ResultTypeNone},
			},
		)
		if !ts.NoError(err, "expected success reading last line") {
			return
		}

		if !ts.Truef(bytes.Contains(response, []byte("END")), "found a value stored on node: %d", i) {
			return
		}
	}
}

// TestClusterAddCommandWithInvalidKey - tests the cluster add command with invalid keys
func (ts *zencachedTestSuite) TestClusterAddCommandWithInvalidKey() {

	ts.genericInvalidKeyTests(
		func(route, path, key []byte) []error {
			value := []byte("value")
			_, errs := ts.instance.ClusterAdd(path, key, value, defaultTTL)
			return errs
		},
	)
}

// TestClusterSetCommandWithInvalidKey - tests the cluster set command with invalid keys
func (ts *zencachedTestSuite) TestClusterSetCommandWithInvalidKey() {

	ts.genericInvalidKeyTests(
		func(route, path, key []byte) []error {
			value := []byte("value")
			_, errs := ts.instance.ClusterSet(path, key, value, defaultTTL)
			return errs
		},
	)
}

// TestClusterGetCommandWithInvalidKey - tests the cluster get command with invalid keys
func (ts *zencachedTestSuite) TestClusterGetCommandWithInvalidKey() {

	ts.genericInvalidKeyTests(
		func(route, path, key []byte) []error {
			_, errs := ts.instance.ClusterGet(path, key)
			return errs
		},
	)
}

// TestClusterDeleteCommandWithInvalidKey - tests the cluster delete command with invalid keys
func (ts *zencachedTestSuite) TestClusterDeleteCommandWithInvalidKey() {

	ts.genericInvalidKeyTests(
		func(route, path, key []byte) []error {
			_, errs := ts.instance.ClusterDelete(path, key)
			return errs
		},
	)
}
