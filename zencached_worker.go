package zencached

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"sync/atomic"

	"github.com/rnojiri/logh"
)

type cmdResponse struct {
	exists   bool
	response []byte
	err      error
}

type cmdJob struct {
	cmd                              MemcachedCommand
	beginResponseSet, endResponseSet memcachedResponseSet
	renderedCmd, path, key           []byte
	forceCacheMissMetric             bool
	response                         chan cmdResponse
}

type nodeWorkers struct {
	logger           *logh.ContextualLogger
	connected        uint32
	numUsedResources uint32
	node             Node
	jobs             chan cmdJob
	rebalanceChannel chan<- struct{}
	configuration    *Configuration
}

// NewTelnetFromNode - creates a new telnet connection based in this node configuration
func (nw *nodeWorkers) NewTelnetFromNode() (*Telnet, error) {

	t, err := NewTelnet(nw.node, nw.configuration.TelnetConfiguration)
	if err != nil {
		if logh.ErrorEnabled {
			nw.logger.Error().Str("host", nw.node.Host).Int("port", nw.node.Port).Err(err).Msg("error creating telnet")
		}

		return nil, err
	}

	err = t.Connect()
	if err != nil {
		if logh.ErrorEnabled {
			nw.logger.Error().Str("host", nw.node.Host).Int("port", nw.node.Port).Err(err).Msg("error connecting to host")
		}

		return nil, err
	}

	return t, nil
}

// terminate - closes all connections
func (nw *nodeWorkers) terminate() {

	close(nw.jobs)
}

func (nw *nodeWorkers) work(telnetConn *Telnet, workerID int) {

	for job := range nw.jobs {

		response := nw.sendAndReadResponse(telnetConn, job.beginResponseSet, job.endResponseSet, job.renderedCmd)

		job.response <- response

		if errors.Is(response.err, ErrNoAvailableConnections) ||
			errors.Is(response.err, ErrMaxReconnectionsReached) ||
			errors.Is(response.err, ErrMemcachedNoResponse) {

			if nw.configuration.RebalanceOnDisconnection {
				nw.rebalanceChannel <- struct{}{}
				break
			}
		}
	}

	atomic.StoreUint32(&nw.connected, 0)
	telnetConn.Close()

	if logh.InfoEnabled {
		nw.logger.Info().Msgf("terminating worker id: %d", workerID)
	}
}

func (z *Zencached) createNodeWorker(node Node, rebalanceChannel chan<- struct{}) (*nodeWorkers, error) {

	nw := &nodeWorkers{
		logger:           logh.CreateContextualLogger("pkg", "zencached", "node", node.String()),
		node:             node,
		connected:        1,
		numUsedResources: 0,
		jobs:             make(chan cmdJob, z.configuration.CommandExecutionBufferSize+1),
		configuration:    z.configuration,
		rebalanceChannel: rebalanceChannel,
	}

	for i := 0; i < int(z.configuration.NumConnectionsPerNode); i++ {

		telnetConn, err := nw.NewTelnetFromNode()
		if err != nil {
			nw.terminate()
			return nil, err
		}

		go nw.work(telnetConn, i)
	}

	return nw, nil
}

// GetNodeWorkersByIndex - returns a telnet connection by node index
func (z *Zencached) GetNodeWorkersByIndex(index int) (nw *nodeWorkers, err error) {

	if atomic.LoadUint32(&z.nodeWorkerArray[index].connected) == 0 {
		return nil, ErrTelnetConnectionIsClosed
	}

	return z.nodeWorkerArray[index], nil
}

// GetConnectedNodeWorkers - returns an idle telnet connection
func (z *Zencached) GetConnectedNodeWorkers(routerHash, path, key []byte) (nw *nodeWorkers, index int, err error) {

	if atomic.LoadUint32(&z.notAvailable) == 1 {
		return nil, 0, ErrNoAvailableNodes
	}

	if routerHash == nil && len(path) > 0 && len(key) > 0 {
		routerHash = append(path, key...)
	}

	if len(routerHash) == 0 {
		index = rand.Intn(len(z.nodeWorkerArray))
	} else {
		index = int(routerHash[len(routerHash)-1]) % len(z.nodeWorkerArray)
	}

	nw, err = z.GetNodeWorkersByIndex(index)

	return
}

// checkResponse - checks the memcached response
func (nw *nodeWorkers) checkResponse(
	telnetConn *Telnet,
	checkReadSet, checkResponseSet memcachedResponseSet,
) cmdResponse {

	response, err := telnetConn.Read(checkReadSet)
	if err != nil {
		return cmdResponse{false, nil, err}
	}

	if len(response) == 0 {
		return cmdResponse{false, nil, ErrMemcachedNoResponse}
	}

	if !bytes.HasPrefix(response, checkResponseSet[0]) {
		if !bytes.Contains(response, checkResponseSet[1]) {
			return cmdResponse{false, nil, fmt.Errorf("%w: %s", ErrMemcachedInvalidResponse, string(response))}
		}

		return cmdResponse{false, response, nil}
	}

	return cmdResponse{true, response, nil}
}

func (nw *nodeWorkers) sendAndReadResponse(
	telnetConn *Telnet,
	beginResponseSet, endResponseSet memcachedResponseSet,
	renderedCmd []byte,
) cmdResponse {

	err := telnetConn.Send(renderedCmd)
	if err == nil {
		return nw.checkResponse(telnetConn, beginResponseSet, endResponseSet)
	}

	return cmdResponse{false, nil, err}
}
