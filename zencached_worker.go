package zencached

import (
	"errors"
	"math/rand"
	"strconv"
	"sync/atomic"

	"github.com/rnojiri/logh"
)

type cmdResponse struct {
	resultType ResultType
	response   []byte
	err        error
}

type cmdJob struct {
	cmd                    MemcachedCommand
	responseSet            TelnetResponseSet
	renderedCmd, path, key []byte
	forceCacheMissMetric   bool
	response               chan cmdResponse
}

type nodeWorkers struct {
	logger           *logh.ContextualLogger
	connected        atomic.Bool
	resources        *resourceChannel
	node             Node
	jobs             chan cmdJob
	rebalanceChannel chan<- struct{}
	configuration    *Configuration
}

type nodeWorkersByNodeName []*nodeWorkers

func (u nodeWorkersByNodeName) Len() int {

	return len(u)
}
func (u nodeWorkersByNodeName) Swap(i, j int) {

	u[i], u[j] = u[j], u[i]
}
func (u nodeWorkersByNodeName) Less(i, j int) bool {

	return u[i].node.Host < u[j].node.Host
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
	nw.resources.terminate()
}

func (nw *nodeWorkers) sendNodeTimedMetrics() {

	nw.configuration.ZencachedMetricsCollector.NumResourcesChangeEvent(nw.node.Host, uint32(nw.resources.numAvailableResources()))
}

func (nw *nodeWorkers) work(telnetConn *Telnet, workerID int) {

	for job := range nw.jobs {

		response := nw.sendAndReadResponse(telnetConn, job.responseSet, job.renderedCmd)

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

	nw.connected.Store(false)

	telnetConn.Close()

	if logh.InfoEnabled {
		nw.logger.Info().Msgf("terminating worker id: %d", workerID)
	}
}

func (z *Zencached) createNodeWorker(node Node, rebalanceChannel chan<- struct{}) (*nodeWorkers, error) {

	nw := &nodeWorkers{
		logger:           logh.CreateContextualLogger("pkg", "zencached", "node", node.String()),
		node:             node,
		connected:        atomic.Bool{},
		resources:        newResource(z.configuration.CommandExecutionBufferSize),
		jobs:             make(chan cmdJob, z.configuration.CommandExecutionBufferSize),
		configuration:    z.configuration,
		rebalanceChannel: rebalanceChannel,
	}

	nw.connected.Store(true)

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

	if !z.nodeWorkerArray[index].connected.Load() {
		return nil, ErrTelnetConnectionIsClosed
	}

	return z.nodeWorkerArray[index], nil
}

// GetConnectedNodeWorkers - returns an idle telnet connection
func (z *Zencached) GetConnectedNodeWorkers(routerHash, path, key []byte) (nw *nodeWorkers, index int, err error) {

	if z.notAvailable.Load() {
		return nil, 0, ErrNoAvailableNodes
	}

	if len(routerHash) == 0 {

		if len(key) > 0 {

			routerHash = []byte{key[len(key)-1]}

		} else if len(path) > 0 {

			routerHash = []byte{path[len(path)-1]}

		} else {

			routerHash = []byte(strconv.Itoa(rand.Intn(len(z.nodeWorkerArray))))
		}
	}

	index = int(routerHash[len(routerHash)-1]) % len(z.nodeWorkerArray)

	nw, err = z.GetNodeWorkersByIndex(index)

	return
}

// checkResponse - checks the memcached response
func (nw *nodeWorkers) checkResponse(
	telnetConn *Telnet,
	responseSet TelnetResponseSet,
) cmdResponse {

	response, resultType, err := telnetConn.Read(responseSet)
	if err != nil {
		return cmdResponse{resultType, nil, err}
	}

	if len(response) == 0 {
		return cmdResponse{resultType, nil, ErrMemcachedNoResponse}
	}

	// if !bytes.HasPrefix(response, checkResponseSet.ResponseSets[0]) {
	// 	if !bytes.Contains(response, checkResponseSet.ResponseSets[1]) {
	// 		return cmdResponse{resultType, nil, fmt.Errorf("%w: %s", ErrMemcachedInvalidResponse, string(response))}
	// 	}

	// 	return cmdResponse{resultType, response, nil}
	// }

	return cmdResponse{resultType, response, nil}
}

func (nw *nodeWorkers) sendAndReadResponse(
	telnetConn *Telnet,
	responseSet TelnetResponseSet,
	renderedCmd []byte,
) cmdResponse {

	err := telnetConn.Send(renderedCmd)
	if err == nil {
		return nw.checkResponse(telnetConn, responseSet)
	}

	return cmdResponse{ResultTypeNone, nil, err}
}
