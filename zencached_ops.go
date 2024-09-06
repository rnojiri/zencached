package zencached

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"time"
)

//
// This file has all implemented commands from memcached.
// More information here:
// https://github.com/memcached/memcached/blob/master/doc/protocol.txt
// author: rnojiri
//

// memcached commands and constants
const (
	lineBreaksR byte = '\r'
	lineBreaksN byte = '\n'
	whiteSpace  byte = ' '
	zero        byte = '0'

	maxAcceptedKeyLength int = 250
)

// OperationResult - default operation results with metadata
type OperationResult struct {

	// Type - returns if the data was already stored
	Type ResultType

	// Data - the key content in bytes
	Data []byte

	// Node - node metadata
	Node Node
}

type TelnetResponseSet struct {
	ResponseSets [][]byte
	ResultTypes  []ResultType
}

// memcached responses
var (
	doubleBreaks []byte = []byte{lineBreaksR, lineBreaksN}
	// responses
	mcrVersion   []byte = []byte("VERSION")
	mcrValue     []byte = []byte("VALUE")
	mcrStored    []byte = append([]byte("STORED"), doubleBreaks...)
	mcrNotStored []byte = append([]byte("NOT_STORED"), doubleBreaks...)
	mcrEnd       []byte = append([]byte("END"), doubleBreaks...)
	mcrNotFound  []byte = append([]byte("NOT_FOUND"), doubleBreaks...)
	mcrDeleted   []byte = append([]byte("DELETED"), doubleBreaks...)

	// response sets
	mcrStoredResponseSet TelnetResponseSet = TelnetResponseSet{
		ResponseSets: [][]byte{mcrNotStored, mcrStored},
		ResultTypes:  []ResultType{ResultTypeNotStored, ResultTypeStored},
	}

	mcrGetCheckBeginResponseSet TelnetResponseSet = TelnetResponseSet{
		ResponseSets: [][]byte{mcrEnd},
		ResultTypes:  []ResultType{ResultTypeFound, ResultTypeNotFound},
	}

	mcrDeletedResponseSet TelnetResponseSet = TelnetResponseSet{
		ResponseSets: [][]byte{mcrDeleted, mcrNotFound},
		ResultTypes:  []ResultType{ResultTypeDeleted, ResultTypeNotFound},
	}

	mcrVersionResponseSet TelnetResponseSet = TelnetResponseSet{
		ResponseSets: [][]byte{mcrVersion},
		ResultTypes:  []ResultType{ResultTypeFound},
	}

	errNoResourceAvailable cmdResponse = cmdResponse{
		resultType: ResultTypeError,
		response:   nil,
		err:        ErrNoAvailableResources,
	}

	// ErrInvalidKeyFormat - raised when a key has a invlid format
	ErrInvalidKeyFormat error = errors.New("key format is invalid, it should have more than 0 or maximum of 250 bytes without new lines and spaces")
)

// MemcachedCommand type
type MemcachedCommand []byte

var (
	// Add - add some key if it not exists
	Add MemcachedCommand = MemcachedCommand("add")

	// Set - sets a key if it exists or not
	Set MemcachedCommand = MemcachedCommand("set")

	// Get - return a key if it exists or not
	Get MemcachedCommand = MemcachedCommand("get")

	// Delete - deletes a key if it exists or not
	Delete MemcachedCommand = MemcachedCommand("delete")

	// Version - returns the server version
	Version MemcachedCommand = MemcachedCommand("version")
)

// ValidateKey - validates a key
func ValidateKey(path, key []byte) error {

	var keyComposite []byte

	if len(path) > 0 {

		keyComposite = path

		if len(key) > 0 {
			keyComposite = append(keyComposite, key...)
		}

	} else {

		keyComposite = key
	}

	if len(keyComposite) == 0 ||
		len(keyComposite) > maxAcceptedKeyLength ||
		bytes.IndexByte(keyComposite, whiteSpace) > -1 ||
		bytes.IndexByte(keyComposite, lineBreaksN) > -1 ||
		bytes.IndexByte(keyComposite, lineBreaksR) > -1 {

		return fmt.Errorf("%w: %s", ErrInvalidKeyFormat, string(keyComposite))
	}

	return nil
}

func (z *Zencached) compressIfConfigured(value []byte) ([]byte, error) {

	var tvalue []byte
	var err error

	if z.compressionEnabled {

		tvalue, err = z.dataCompressor.Compress(value)
		if err != nil {
			return nil, err
		}

	} else {

		tvalue = value
	}

	return tvalue, nil
}

// renderStoreCmd - like Sprintf, but in bytes
func (z *Zencached) renderStoreCmd(cmd MemcachedCommand, path, key, value []byte, ttl uint64) ([]byte, error) {

	length := strconv.Itoa(len(value))

	ttlStr := strconv.FormatUint(ttl, 10)

	buffer := bytes.Buffer{}
	buffer.Grow(len(cmd) + len(path) + len(key) + len(value) + len(ttlStr) + len(length) + 4 + (len(doubleBreaks) * 2) + 1)
	buffer.Write(cmd)
	buffer.WriteByte(whiteSpace)
	buffer.Write(path)
	buffer.Write(key)
	buffer.WriteByte(whiteSpace)
	buffer.WriteByte(zero)
	buffer.WriteByte(whiteSpace)
	buffer.WriteString(ttlStr)
	buffer.WriteByte(whiteSpace)
	buffer.WriteString(length)
	buffer.Write(doubleBreaks)
	buffer.Write(value)
	buffer.Write(doubleBreaks)

	return buffer.Bytes(), nil
}

// Set - performs an storage set operation
func (z *Zencached) Set(routerHash, path, key, value []byte, ttl uint64) (*OperationResult, error) {

	if err := ValidateKey(path, key); err != nil {
		return nil, err
	}

	return z.store(Set, routerHash, path, key, value, ttl)
}

// Add - performs an storage add operation
func (z *Zencached) Add(routerHash, path, key, value []byte, ttl uint64) (*OperationResult, error) {

	if err := ValidateKey(path, key); err != nil {
		return nil, err
	}

	return z.store(Add, routerHash, path, key, value, ttl)
}

// store - generic store
func (z *Zencached) store(cmd MemcachedCommand, routerHash, path, key, value []byte, ttl uint64) (*OperationResult, error) {

	nw, _, err := z.GetConnectedNodeWorkers(routerHash, path, key)
	if err != nil {
		return nil, err
	}

	return z.baseStore(nw, cmd, path, key, value, ttl)
}

func (z *Zencached) addJobAndWait(nw *nodeWorkers, job cmdJob) cmdResponse {

	hasResources := nw.resources.take()

	if !hasResources {
		return errNoResourceAvailable
	}

	defer func() {
		nw.resources.put()
	}()

	var result cmdResponse

	if !z.metricsEnabled {

		nw.jobs <- job
		result = <-job.response

	} else {

		start := time.Now()

		nw.jobs <- job
		result = <-job.response

		nw.configuration.ZencachedMetricsCollector.CommandExecutionElapsedTime(
			nw.node.Host,
			job.cmd, job.path, job.key,
			time.Since(start).Nanoseconds(),
		)

		nw.configuration.ZencachedMetricsCollector.CommandExecution(nw.node.Host, job.cmd, job.path, job.key)

		if result.resultType == ResultTypeFound && !job.forceCacheMissMetric {
			nw.configuration.ZencachedMetricsCollector.CacheHitEvent(nw.node.Host, job.cmd, job.path, job.key)
		} else {
			nw.configuration.ZencachedMetricsCollector.CacheMissEvent(nw.node.Host, job.cmd, job.path, job.key)
		}

		if result.err != nil {

			nw.configuration.ZencachedMetricsCollector.CommandExecutionError(
				nw.node.Host,
				job.cmd, job.path, job.key,
				result.err,
			)
		}
	}

	return result
}

// baseStore - base storage function
func (z *Zencached) baseStore(nw *nodeWorkers, cmd MemcachedCommand, path, key, value []byte, ttl uint64) (*OperationResult, error) {

	tvalue, err := z.compressIfConfigured(value)
	if err != nil {
		return &OperationResult{
			Type: ResultTypeCompressionError,
			Data: value,
			Node: nw.node,
		}, err
	}

	renderedCmd, err := z.renderStoreCmd(cmd, path, key, tvalue, ttl)
	if err != nil {
		return &OperationResult{
			Type: ResultTypeError,
			Node: nw.node,
		}, err
	}

	job := cmdJob{
		cmd:                  cmd,
		responseSet:          mcrStoredResponseSet,
		renderedCmd:          renderedCmd,
		path:                 path,
		key:                  key,
		forceCacheMissMetric: true,
		response:             make(chan cmdResponse),
	}

	result := z.addJobAndWait(nw, job)
	if result.err != nil {
		return &OperationResult{
			Type: ResultTypeError,
			Node: nw.node,
		}, result.err
	}

	return &OperationResult{
		Type: result.resultType,
		Node: nw.node,
	}, nil
}

// renderKeyOnlyCmd - like Sprintf, but in bytes
func (z *Zencached) renderKeyOnlyCmd(cmd MemcachedCommand, path, key []byte) []byte {

	buffer := bytes.Buffer{}
	buffer.Grow(len(cmd) + len(path) + len(key) + 1 + len(doubleBreaks))
	buffer.Write(cmd)
	buffer.WriteByte(whiteSpace)
	buffer.Write(path)
	buffer.Write(key)
	buffer.Write(doubleBreaks)

	return buffer.Bytes()
}

// Get - performs a get operation
func (z *Zencached) Get(routerHash, path, key []byte) (*OperationResult, error) {

	if err := ValidateKey(path, key); err != nil {
		return nil, err
	}

	nw, _, err := z.GetConnectedNodeWorkers(routerHash, path, key)
	if err != nil {
		return nil, err
	}

	return z.baseGet(nw, path, key)
}

// extractGetCmdValue - extracts a value from the response
func (z *Zencached) extractGetCmdValue(response []byte) ([]byte, error) {

	i := 0
	end := true

	for ; i < len(mcrEnd); i++ {

		if response[i] != mcrEnd[i] {
			end = false
			break
		}
	}

	if end {
		return nil, nil
	}

	for ; i < len(mcrValue); i++ {

		if response[i] != mcrValue[i] {
			return nil, fmt.Errorf("no value protocol found")
		}
	}

	if response[i] != whiteSpace {
		return nil, fmt.Errorf("expecting a white space before the key")
	}

	i++

	found := false

	// ignore the key
	for ; i < len(response); i++ {
		if response[i] == whiteSpace {
			found = true
			i++
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("expecting a white space after the key")
	}

	// ignore the protocol flag
	for ; i < len(response); i++ {
		if response[i] == whiteSpace {
			found = true
			i++
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("expecting a white space after the protocol flag")
	}

	found = false
	startLen := i
	endLen := 0

	for ; i < len(response); i++ {
		if (response[i] == lineBreaksN || response[i] == lineBreaksR) && (response[i+1] == lineBreaksN || response[i+1] == lineBreaksR) {
			endLen = i
			i += 2
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("expecting a line break after the value length")
	}

	valueLen, err := strconv.Atoi(string(response[startLen:endLen]))
	if err != nil {
		return nil, fmt.Errorf("error converting value length in number: %s", err)
	}

	return response[i : i+valueLen], nil
}

func (z *Zencached) decompressIfConfigured(value []byte) ([]byte, error) {

	var tvalue []byte
	var err error

	if z.compressionEnabled {

		tvalue, err = z.dataCompressor.Decompress(value)
		if err != nil {
			return nil, err
		}

	} else {

		tvalue = value
	}

	return tvalue, nil
}

// baseGet - performs a get operation
func (z *Zencached) baseGet(nw *nodeWorkers, path, key []byte) (*OperationResult, error) {

	job := cmdJob{
		cmd:                  Get,
		responseSet:          mcrGetCheckBeginResponseSet,
		renderedCmd:          z.renderKeyOnlyCmd(Get, path, key),
		path:                 path,
		key:                  key,
		forceCacheMissMetric: false,
		response:             make(chan cmdResponse),
	}

	result := z.addJobAndWait(nw, job)
	if result.err != nil {
		return &OperationResult{
			Type: ResultTypeError,
			Node: nw.node,
		}, result.err
	}

	if result.resultType != ResultTypeFound {
		return &OperationResult{
			Type: result.resultType,
			Node: nw.node,
		}, nil
	}

	extractedValue, err := z.extractGetCmdValue(result.response)
	if err != nil {
		return &OperationResult{
			Type: ResultTypeError,
			Node: nw.node,
		}, err
	}

	if len(extractedValue) == 0 {
		return &OperationResult{
			Type: ResultTypeNotFound,
			Data: nil,
			Node: nw.node,
		}, nil
	}

	responseData, err := z.decompressIfConfigured(extractedValue)
	if err != nil {
		return &OperationResult{
			Type: ResultTypeDecompressionError,
			Data: extractedValue,
			Node: nw.node,
		}, err
	}

	return &OperationResult{
		Type: ResultTypeFound,
		Data: responseData,
		Node: nw.node,
	}, nil
}

// Delete - performs a delete operation
func (z *Zencached) Delete(routerHash, path, key []byte) (*OperationResult, error) {

	if err := ValidateKey(path, key); err != nil {
		return nil, err
	}

	nw, _, err := z.GetConnectedNodeWorkers(routerHash, path, key)
	if err != nil {
		return nil, err
	}

	return z.baseDelete(nw, path, key)
}

// baseDelete - performs a delete operation
func (z *Zencached) baseDelete(nw *nodeWorkers, path, key []byte) (*OperationResult, error) {

	job := cmdJob{
		cmd:                  Delete,
		responseSet:          mcrDeletedResponseSet,
		renderedCmd:          z.renderKeyOnlyCmd(Delete, path, key),
		path:                 path,
		key:                  key,
		forceCacheMissMetric: false,
		response:             make(chan cmdResponse),
	}

	result := z.addJobAndWait(nw, job)
	if result.err != nil {
		return nil, result.err
	}

	return &OperationResult{
		Type: result.resultType,
		Node: nw.node,
	}, nil
}

// renderNoParameterCmd - like Sprintf, but in bytes
func (z *Zencached) renderNoParameterCmd(cmd MemcachedCommand) []byte {

	buffer := bytes.Buffer{}
	buffer.Grow(len(cmd) + len(doubleBreaks))
	buffer.Write(cmd)
	buffer.Write(doubleBreaks)

	return buffer.Bytes()
}

// extractVersionCmdValue - extracts a value from the response
func (z *Zencached) extractVersionCmdValue(response []byte) ([]byte, error) {

	i := 0

	for ; i < len(mcrVersion); i++ {

		if response[i] != mcrVersion[i] {
			return nil, fmt.Errorf("no version protocol found")
		}
	}

	if response[i] != whiteSpace {
		return nil, fmt.Errorf("expecting a white space before the version number")
	}

	i++
	start := i
	end := 0

	for ; i < len(response); i++ {
		if (response[i] == lineBreaksN || response[i] == lineBreaksR) && (response[i+1] == lineBreaksN || response[i+1] == lineBreaksR) {
			end = i
			break
		}
	}

	if end == 0 {
		return nil, fmt.Errorf("no endline found after the version string")
	}

	return response[start:end], nil
}

// Version - performs a version operation
func (z *Zencached) Version(routerHash []byte) (*OperationResult, error) {

	nw, _, err := z.GetConnectedNodeWorkers(routerHash, nil, nil)
	if err != nil {
		return nil, err
	}

	return z.baseVersion(nw)
}

// baseVersion - performs a get operation
func (z *Zencached) baseVersion(nw *nodeWorkers) (*OperationResult, error) {

	job := cmdJob{
		cmd:                  Version,
		responseSet:          mcrVersionResponseSet,
		renderedCmd:          z.renderNoParameterCmd(Version),
		forceCacheMissMetric: true,
		response:             make(chan cmdResponse),
	}

	result := z.addJobAndWait(nw, job)
	if result.err != nil {
		return nil, result.err
	}

	if result.resultType != ResultTypeFound {
		return &OperationResult{
			Type: result.resultType,
			Node: nw.node,
		}, nil
	}

	response, err := z.extractVersionCmdValue(result.response)
	if err != nil {
		return &OperationResult{
			Type: ResultTypeError,
			Node: nw.node,
		}, err
	}

	return &OperationResult{
		Type: result.resultType,
		Data: response,
		Node: nw.node,
	}, nil
}
