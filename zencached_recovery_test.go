package zencached_test

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/rnojiri/dockerh"
	"github.com/rnojiri/logh"
	"github.com/rnojiri/zencached"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type zencachedRecoveryTestSuite struct {
	suite.Suite
	instance *zencached.Zencached
	config   *zencached.Configuration
}

func (ts *zencachedRecoveryTestSuite) SetupSuite() {

	logh.ConfigureGlobalLogger(logh.DEBUG, logh.CONSOLE)

	terminatePods()

	nodes := startMemcachedCluster()

	var err error
	ts.instance, ts.config, err = createZencached(nodes, true, nil)
	if err != nil {
		ts.T().Fatalf("expected no errors creating zencached: %v", err)
	}
}

func (ts *zencachedRecoveryTestSuite) TearDownSuite() {

	ts.instance.Shutdown()

	terminatePods()
}

func (ts *zencachedRecoveryTestSuite) loopCommands(exitLoop chan struct{}) {

	isErrMaxCommandRetriesReached := func(err error) bool {
		return ts.True(errors.Is(err, zencached.ErrMaxCommandRetriesReached), fmt.Sprintf("only expected error is the max command retries reached, instead: %s", err))
	}

	i := 0
	for {
		select {
		case <-exitLoop:
			return
		default:
			path := []byte{'p'}
			key := []byte(strconv.Itoa(i))

			_, _, err := ts.instance.Get(nil, path, key)
			if err != nil && !isErrMaxCommandRetriesReached(err) {
				return
			}

			_, err = ts.instance.Set(nil, path, key, key, defaultTTL)
			if err != nil && !isErrMaxCommandRetriesReached(err) {
				return
			}

			_, err = ts.instance.Delete(nil, path, key)
			if err != nil && !isErrMaxCommandRetriesReached(err) {
				return
			}

			i++
			<-time.After(10 * time.Millisecond)
		}
	}
}

// TestClusterRebalanceRemovingNode - tests the cluster rebalance function removing a node
func (ts *zencachedRecoveryTestSuite) TestClusterRebalanceRemovingNode() {

	defer func() {
		ts.config.NodeListFunction = nil
	}()

	exitLoop := make(chan struct{}, 1)

	go ts.loopCommands(exitLoop)

	var original, minusTwo []zencached.Node

	ts.config.NodeListFunction = func() ([]zencached.Node, error) {

		original = ts.instance.GetConnectedNodes()
		minusTwo = original[0 : len(original)-2]

		return minusTwo, nil
	}

	err := ts.instance.Rebalance()
	if !ts.NoError(err, "expected no error after rebalancing") {
		return
	}

	after := ts.instance.GetConnectedNodes()
	ts.ElementsMatch(minusTwo, after, "expected same nodes")

	ts.config.NodeListFunction = func() ([]zencached.Node, error) {

		return original, nil
	}

	err = ts.instance.Rebalance()
	if !ts.NoError(err, "expected no error after rebalancing") {
		return
	}

	after = ts.instance.GetConnectedNodes()
	ts.ElementsMatch(original, after, "expected same nodes")

	<-time.After(100 * time.Millisecond)
	exitLoop <- struct{}{}
	<-time.After(1 * time.Second)
}

// TestClusterRebalanceAddingNode - tests the cluster rebalance function adding a node
func (ts *zencachedRecoveryTestSuite) TestClusterRebalanceAddingNode() {

	defer func() {
		ts.config.NodeListFunction = nil
	}()

	exitLoop := make(chan struct{}, 1)

	go ts.loopCommands(exitLoop)

	original := ts.instance.GetConnectedNodes()

	newPodName, newNode := createExtraMemcachedPod(ts.T())
	defer dockerh.Remove(newPodName)

	newNodeConf := append(original, newNode)

	ts.config.NodeListFunction = func() ([]zencached.Node, error) {

		return newNodeConf, nil
	}

	err := ts.instance.Rebalance()
	if !ts.NoError(err, "expected no error after rebalancing") {
		return
	}

	after := ts.instance.GetConnectedNodes()
	ts.ElementsMatch(newNodeConf, after, "expected same nodes")

	ts.config.NodeListFunction = func() ([]zencached.Node, error) {

		return original, nil
	}

	err = ts.instance.Rebalance()
	if !ts.NoError(err, "expected no error after rebalancing") {
		return
	}

	after = ts.instance.GetConnectedNodes()
	ts.ElementsMatch(original, after, "expected same nodes")

	<-time.After(100 * time.Millisecond)
	exitLoop <- struct{}{}
	<-time.After(1 * time.Second)
}

// TestClusterNodeDown - tests the cluster  recovery when a node is down
func (ts *zencachedRecoveryTestSuite) TestClusterNodeDown() {

	_, _, err := ts.instance.Get([]byte{10, 199}, []byte("p"), []byte("k"))
	if !ts.NoError(err, "expected no error executing get in node zero") {
		return
	}

	err = dockerh.Remove(memcachedPodNames[1])
	if !ts.NoError(err, fmt.Sprintf("expected no error removing pod: %s", memcachedPodNames[1])) {
		return
	}

	_, _, err = ts.instance.Get([]byte{10, 199}, []byte("p"), []byte("k"))
	if !ts.True(errors.Is(err, zencached.ErrMaxCommandRetriesReached), fmt.Sprintf("expected an error describing max command retries, instead: %v", err)) {
		return
	}

	_, err = dockerh.CreateMemcached(memcachedPodNames[1], memcachedPodPort[1], 64)
	if !ts.NoError(err, "expected no error creating the memcached pod") {
		return
	}

	err = ts.instance.Rebalance()
	if !ts.NoError(err, "expected no error rebalancing nodes") {
		return
	}

	_, _, err = ts.instance.Get([]byte{10, 199}, []byte("p"), []byte("k"))
	if !ts.NoError(err, "expected no error executing get in node zero after it gets back") {
		return
	}
}

// TestClusterAllNodesDown - tests the cluster when all nodes are down
func (ts *zencachedRecoveryTestSuite) TestClusterAllNodesDown() {

	_, _, err := ts.instance.Get([]byte{10, 199}, []byte("p"), []byte("k"))
	if !ts.NoError(err, "expected no error executing get in node zero") {
		return
	}

	terminatePods()

	_, _, err = ts.instance.Get(nil, []byte("p"), []byte("k"))
	if !ts.True(errors.Is(err, zencached.ErrMaxCommandRetriesReached), "expected an error describing max command retries") {
		return
	}

	err = ts.instance.Rebalance()
	if !ts.NoError(err, "expected no error rebalancing nodes") {
		return
	}

	for i := 0; i < 100; i++ {
		_, _, err = ts.instance.Get(nil, []byte("p"), []byte(fmt.Sprintf("k%d", i)))
		if !ts.True(errors.Is(err, zencached.ErrNoAvailableNodes), "expected an error describing that there are no nodes available") {
			return
		}
	}

	startMemcachedCluster()

	err = ts.instance.Rebalance()
	if !ts.NoError(err, "expected no error rebalancing nodes") {
		return
	}

	for i := 0; i < 100; i++ {
		_, _, err = ts.instance.Get(nil, []byte("p"), []byte(fmt.Sprintf("k%d", i)))
		if !ts.NoError(err, "expected no errors after rebalancing") {
			return
		}
	}
}

func TestZencachedRecoveryTestSuite(t *testing.T) {

	suite.Run(t, new(zencachedRecoveryTestSuite))
}

// TestMaxConnectionRetryError - tests when the maximum number of connections is reached
func TestMaxConnectionRetryError(t *testing.T) {

	newPodName, newNode := createExtraMemcachedPod(t)

	instance, _, err := createZencached([]zencached.Node{newNode}, false, nil)
	if err != nil {
		t.Fatalf("expected no errors creating zencached: %v", err)
	}

	_, _, err = instance.Get(nil, []byte("p"), []byte("k"))
	if !assert.NoError(t, err, "expected no error connecting") {
		return
	}

	dockerh.Remove(newPodName)

	_, _, err = instance.Get(nil, []byte("p"), []byte("k"))
	if !assert.True(t, errors.Is(err, zencached.ErrMaxCommandRetriesReached), "expected max command retries reached error") {
		return
	}

	createExtraMemcachedPod(t)
	defer dockerh.Remove(newPodName)

	_, _, err = instance.Get(nil, []byte("p"), []byte("k"))
	if !assert.NoError(t, err, "expected no error reconnecting") {
		return
	}

	instance.Shutdown()
}
