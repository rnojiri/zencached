package zencached_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
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
	ts.instance, ts.config, err = createZencached(nodes, 50, false, nil, nil, zencached.CompressionTypeNone)
	if err != nil {
		ts.T().Fatalf("expected no errors creating zencached: %v", err)
	}
}

func (ts *zencachedRecoveryTestSuite) SetupTest() {

	ts.instance.Rebalance()
}

func (ts *zencachedRecoveryTestSuite) TearDownSuite() {

	ts.instance.Shutdown()

	terminatePods()
}

func isDisconnectionError(t *testing.T, err error) bool {

	return assert.True(
		t,
		errors.Is(err, zencached.ErrMemcachedNoResponse) ||
			errors.Is(err, zencached.ErrNoAvailableNodes) ||
			errors.Is(err, zencached.ErrConnectionWrite) ||
			errors.Is(err, zencached.ErrConnectionRead) ||
			errors.Is(err, zencached.ErrTelnetConnectionIsClosed),
		fmt.Sprintf("expected a disconnection error, instead: %v", err),
	)
}

func (ts *zencachedRecoveryTestSuite) loopCommands(exitLoop chan struct{}) {

	i := 0
	for {
		select {
		case <-exitLoop:
			return
		default:
			path := []byte{'p'}
			key := []byte(strconv.Itoa(i))

			_, err := ts.instance.Get(context.Background(), nil, path, key)
			if err != nil && !isDisconnectionError(ts.T(), err) {
				return
			}

			_, err = ts.instance.Set(context.Background(), nil, path, key, key, defaultTTL)
			if err != nil && !isDisconnectionError(ts.T(), err) {
				return
			}

			_, err = ts.instance.Delete(context.Background(), nil, path, key)
			if err != nil && !isDisconnectionError(ts.T(), err) {
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

	ts.instance.Rebalance()

	<-time.After(5 * time.Second)

	after := ts.instance.GetConnectedNodes()
	ts.ElementsMatch(minusTwo, after, "expected same nodes")

	ts.config.NodeListFunction = func() ([]zencached.Node, error) {

		return original, nil
	}

	ts.instance.Rebalance()

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

	ts.instance.Rebalance()

	after := ts.instance.GetConnectedNodes()
	ts.ElementsMatch(newNodeConf, after, "expected same nodes")

	ts.config.NodeListFunction = func() ([]zencached.Node, error) {

		return original, nil
	}

	ts.instance.Rebalance()

	after = ts.instance.GetConnectedNodes()
	ts.ElementsMatch(original, after, "expected same nodes")

	<-time.After(100 * time.Millisecond)
	exitLoop <- struct{}{}
	<-time.After(1 * time.Second)
}

// TestClusterNodeDown - tests the cluster  recovery when a node is down
func (ts *zencachedRecoveryTestSuite) TestClusterNodeDown() {

	_, err := ts.instance.Get(context.Background(), []byte{3}, []byte("p"), []byte("k"))
	if !ts.NoError(err, "expected no error executing get in node zero") {
		return
	}

	err = dockerh.Remove(memcachedPodNames[1])
	if !ts.NoError(err, fmt.Sprintf("expected no error removing pod: %s", memcachedPodNames[1])) {
		return
	}

	<-time.After(2 * time.Second)

	_, err = ts.instance.Get(context.Background(), []byte{3}, []byte("p"), []byte("k"))
	if !isDisconnectionError(ts.T(), err) {
		return
	}

	_, err = dockerh.CreateMemcached(memcachedPodNames[1], memcachedPodPort[1], 64, "1m")
	if !ts.NoError(err, "expected no error creating the memcached pod") {
		return
	}

	<-time.After(2 * time.Second)

	ts.instance.Rebalance()

	reconnected := false

	for i := 0; i < 3; i++ {

		_, err = ts.instance.Get(context.Background(), []byte{10, 199}, []byte("p"), []byte("k"))
		if err == nil {
			reconnected = true
			break
		}
	}

	if !reconnected {
		if !ts.NoError(err, fmt.Sprintf("expected no error executing get in node zero after it gets back: %s", err)) {
			return
		}
	}
}

// TestClusterAllNodesDown - tests the cluster when all nodes are down
func (ts *zencachedRecoveryTestSuite) TestClusterAllNodesDown() {

	_, err := ts.instance.Get(context.Background(), []byte{10, 199}, []byte("p"), []byte("k"))
	if !ts.NoError(err, "expected no error executing get in node zero") {
		return
	}

	terminatePods()

	<-time.After(20 * time.Second)

	_, err = ts.instance.Get(context.Background(), nil, []byte("p"), []byte("k"))
	if !isDisconnectionError(ts.T(), err) {
		return
	}

	ts.instance.Rebalance()

	for i := 0; i < 100; i++ {
		_, err = ts.instance.Get(context.Background(), []byte{byte(i)}, []byte("p"), []byte(fmt.Sprintf("k%d", i)))
		if !isDisconnectionError(ts.T(), err) {
			return
		}
	}

	<-time.After(2 * time.Second)

	startMemcachedCluster()

	<-time.After(2 * time.Second)

	ts.instance.Rebalance()

	<-time.After(2 * time.Second)

	for i := 0; i < 100; i++ {
		_, err = ts.instance.Get(context.Background(), nil, []byte("p"), []byte(fmt.Sprintf("k%d", i)))
		if !ts.NoError(err, "expected no errors after rebalancing") {
			return
		}
	}
}

// TestNodeRecreationRecovery - simulates a pod recreation scenario where the checkConnection()
// goroutine exits early after detecting a failure, and verifies that Close() does not deadlock
// when called afterward (goroutine leak bug: connCheckEndChan send would block forever).
func (ts *zencachedRecoveryTestSuite) TestNodeRecreationRecovery() {

	// Find a node bound to 127.0.0.1: these keep their address after container recreation,
	// whereas Docker-bridge nodes get a new IP on each restart making recovery impossible.
	// Derive the pod name from the port (memcachedPodPort[i] = 11211+i).
	var targetNode zencached.Node
	for _, n := range ts.instance.GetConnectedNodes() {
		if n.Host == "127.0.0.1" {
			targetNode = n
			break
		}
	}
	if !ts.NotEmpty(targetNode.Host, "could not find a 127.0.0.1 node in connected nodes") {
		return
	}

	podIndex := targetNode.Port - memcachedPodPort[0]
	targetPod := memcachedPodNames[podIndex]
	targetPort := targetNode.Port

	// Create a dedicated single-node instance so routing always hits index 0 (no ambiguity).
	singleNodeInstance, _, err := createZencached(
		[]zencached.Node{targetNode},
		50,
		false,
		nil,
		nil,
		zencached.CompressionTypeNone,
	)
	if !ts.NoError(err, "expected no error creating single-node zencached") {
		return
	}
	defer singleNodeInstance.Shutdown()

	// Baseline: operations must succeed before the test starts.
	_, err = singleNodeInstance.Get(context.Background(), nil, []byte("p"), []byte("baseline"))
	if !ts.NoError(err, "expected no error on baseline get") {
		return
	}

	// Simulate pod removal (memcached container down — like a Kubernetes pod being recreated).
	err = dockerh.Remove(targetPod)
	if !ts.NoError(err, fmt.Sprintf("expected no error removing pod: %s", targetPod)) {
		return
	}

	// Wait for checkConnection() to detect the failure and exit its goroutine.
	// ConnectionCheckTimeout defaults to 3s; we wait 5s to be safe.
	<-time.After(5 * time.Second)

	// Operations must now return a disconnection error.
	_, err = singleNodeInstance.Get(context.Background(), nil, []byte("p"), []byte("during-down"))
	if !isDisconnectionError(ts.T(), err) {
		return
	}

	// Recreate the pod (simulates Kubernetes bringing the container back).
	_, err = dockerh.CreateMemcached(targetPod, targetPort, 64, "1m")
	if !ts.NoError(err, "expected no error recreating the memcached pod") {
		return
	}

	// Give the new container time to be ready.
	<-time.After(2 * time.Second)

	// Rebalance must complete without deadlocking.
	done := make(chan struct{})
	go func() {
		singleNodeInstance.Rebalance()
		close(done)
	}()

	select {
	case <-done:
		// Rebalance completed — no deadlock
	case <-time.After(15 * time.Second):
		ts.Fail("Rebalance() deadlocked: Close() is likely blocked on connCheckEndChan after checkConnection() goroutine exited early")
		return
	}

	// After rebalance, operations must succeed again.
	recovered := false
	for i := 0; i < 5; i++ {
		_, err = singleNodeInstance.Get(context.Background(), nil, []byte("p"), []byte("after-recovery"))
		if err == nil {
			recovered = true
			break
		}
		<-time.After(500 * time.Millisecond)
	}

	ts.True(recovered, "expected successful get after pod recreation and rebalance")
}

// TestConcurrentDisconnectionDoesNotBlock verifies that when N goroutines fail simultaneously
// on the same connection, reportDisconnection() does not block any of them.
//
// Before the CompareAndSwap fix, multiple goroutines could pass the onFailure.Load() guard
// simultaneously and all try to send to disconnectionChannel. If the channel was full (capacity =
// NumConnectionsPerNode), the extra goroutines would block inside reportDisconnection(),
// causing the entire Get/Set/Delete call to hang forever 
func (ts *zencachedRecoveryTestSuite) TestConcurrentDisconnectionDoesNotBlock() {

	var targetNode zencached.Node
	for _, n := range ts.instance.GetConnectedNodes() {
		if n.Host == "127.0.0.1" {
			targetNode = n
			break
		}
	}
	if !ts.NotEmpty(targetNode.Host, "could not find a 127.0.0.1 node in connected nodes") {
		return
	}

	podIndex := targetNode.Port - memcachedPodPort[0]
	targetPod := memcachedPodNames[podIndex]
	targetPort := targetNode.Port

	const numConns = 3
	singleNodeInstance, _, err := createZencached(
		[]zencached.Node{targetNode},
		100,
		false,
		nil,
		nil,
		zencached.CompressionTypeNone,
	)
	if !ts.NoError(err, "expected no error creating single-node zencached") {
		return
	}
	defer singleNodeInstance.Shutdown()

	_, err = singleNodeInstance.Get(context.Background(), nil, []byte("p"), []byte("concurrent-baseline"))
	if !ts.NoError(err, "expected no error on baseline get") {
		return
	}

	// Stop the node so that all subsequent writes fail immediately with "broken pipe".
	err = dockerh.Remove(targetPod)
	if !ts.NoError(err, fmt.Sprintf("expected no error removing pod: %s", targetPod)) {
		return
	}

	defer func() {
		dockerh.CreateMemcached(targetPod, targetPort, 64, "1m")
	}()

	// Fire more concurrent Gets than the disconnectionChannel capacity (= numConns).
	// With the old code, goroutines would race inside reportDisconnection():
	//   - All pass onFailure.Load() == false simultaneously
	//   - All try to send to the channel (capacity = numConns)
	//   - The (numConns+1)-th goroutine blocks forever → Get() never returns
	// With the CompareAndSwap fix, only one goroutine sends; the rest return immediately.
	concurrency := numConns * 4
	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				singleNodeInstance.Get(context.Background(), nil, []byte("p"), []byte("concurrent-key"))
			}()
		}
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All Get()s returned — reportDisconnection() did not block any goroutine.
	case <-time.After(10 * time.Second):
		ts.Fail("concurrent Get()s blocked: reportDisconnection() likely blocked on a full disconnectionChannel (missing CompareAndSwap guard)")
	}
}

func TestZencachedRecoveryTestSuite(t *testing.T) {

	suite.Run(t, new(zencachedRecoveryTestSuite))
}
