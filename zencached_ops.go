package zencached

import (
	"bytes"
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
)

type memcachedResponseSet [][]byte

// memcached responses
var (
	doubleBreaks []byte = []byte{lineBreaksR, lineBreaksN}
	// responses
	mcrValue     []byte = []byte("VALUE") // the only prefix
	mcrStored    []byte = []byte("STORED")
	mcrNotStored []byte = []byte("NOT_STORED")
	mcrEnd       []byte = []byte("END")
	mcrNotFound  []byte = []byte("NOT_FOUND")
	mcrDeleted   []byte = []byte("DELETED")
	mcrVersion   []byte = []byte("VERSION")

	// response set
	mcrStoredResponseSet        memcachedResponseSet = memcachedResponseSet{mcrStored, mcrNotStored}
	mcrGetCheckBeginResponseSet memcachedResponseSet = memcachedResponseSet{mcrEnd}
	mcrGetCheckEndResponseSet   memcachedResponseSet = memcachedResponseSet{mcrValue, mcrEnd}
	mcrDeletedResponseSet       memcachedResponseSet = memcachedResponseSet{mcrDeleted, mcrNotFound}
	mcrVersionResponseSet       memcachedResponseSet = memcachedResponseSet{mcrVersion}
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

// renderStoreCmd - like Sprintf, but in bytes
func (z *Zencached) renderStoreCmd(cmd MemcachedCommand, path, key, value []byte, ttl uint64) []byte {

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

	return buffer.Bytes()
}

// Set - performs an storage set operation
func (z *Zencached) Set(routerHash, path, key, value []byte, ttl uint64) (bool, error) {

	return z.store(Set, routerHash, path, key, value, ttl)
}

// Add - performs an storage add operation
func (z *Zencached) Add(routerHash, path, key, value []byte, ttl uint64) (bool, error) {

	return z.store(Add, routerHash, path, key, value, ttl)
}

// store - generic store
func (z *Zencached) store(cmd MemcachedCommand, routerHash, path, key, value []byte, ttl uint64) (bool, error) {

	nw, _, err := z.GetConnectedNodeWorkers(routerHash, path, key)
	if err != nil {
		return false, err
	}

	return z.baseStore(nw, cmd, path, key, value, ttl)
}

func (z *Zencached) addJobAndWait(nw *nodeWorkers, job cmdJob) cmdResponse {

	var result cmdResponse

	if z.configuration.ZencachedMetricsCollector == nil {

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

		if result.exists && !job.forceCacheMissMetric {
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
func (z *Zencached) baseStore(nw *nodeWorkers, cmd MemcachedCommand, path, key, value []byte, ttl uint64) (bool, error) {

	job := cmdJob{
		cmd:                  cmd,
		beginResponseSet:     mcrStoredResponseSet,
		endResponseSet:       mcrStoredResponseSet,
		renderedCmd:          z.renderStoreCmd(cmd, path, key, value, ttl),
		path:                 path,
		key:                  key,
		forceCacheMissMetric: true,
		response:             make(chan cmdResponse),
	}

	result := z.addJobAndWait(nw, job)

	return result.exists, result.err
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
func (z *Zencached) Get(routerHash, path, key []byte) ([]byte, bool, error) {

	nw, _, err := z.GetConnectedNodeWorkers(routerHash, path, key)
	if err != nil {
		return nil, false, err
	}

	return z.baseGet(nw, path, key)
}

// extractGetCmdValue - extracts a value from the response
func (z *Zencached) extractGetCmdValue(response []byte) (start, end int, err error) {

	start = -1
	end = -1

	for i := 0; i < len(response); i++ {
		if start == -1 && response[i] == lineBreaksN {
			start = i + 1
		} else if start >= 0 && response[i] == lineBreaksR {
			end = i
			break
		}
	}

	if start == -1 {
		err = fmt.Errorf("no value found")
	}

	if end == -1 {
		end = len(response) - 1
	}

	return
}

// baseGet - performs a get operation
func (z *Zencached) baseGet(nw *nodeWorkers, path, key []byte) ([]byte, bool, error) {

	job := cmdJob{
		cmd:                  Get,
		beginResponseSet:     mcrGetCheckBeginResponseSet,
		endResponseSet:       mcrGetCheckEndResponseSet,
		renderedCmd:          z.renderKeyOnlyCmd(Get, path, key),
		path:                 path,
		key:                  key,
		forceCacheMissMetric: false,
		response:             make(chan cmdResponse),
	}

	result := z.addJobAndWait(nw, job)

	if !result.exists || result.err != nil {
		return nil, false, result.err
	}

	start, end, err := z.extractGetCmdValue([]byte(result.response))
	if err != nil {
		return nil, false, err
	}

	return result.response[start:end], true, nil
}

// Delete - performs a delete operation
func (z *Zencached) Delete(routerHash, path, key []byte) (bool, error) {

	nw, _, err := z.GetConnectedNodeWorkers(routerHash, path, key)
	if err != nil {
		return false, err
	}

	return z.baseDelete(nw, path, key)
}

// baseDelete - performs a delete operation
func (z *Zencached) baseDelete(nw *nodeWorkers, path, key []byte) (bool, error) {

	job := cmdJob{
		cmd:                  Delete,
		beginResponseSet:     mcrDeletedResponseSet,
		endResponseSet:       mcrDeletedResponseSet,
		renderedCmd:          z.renderKeyOnlyCmd(Delete, path, key),
		path:                 path,
		key:                  key,
		forceCacheMissMetric: false,
		response:             make(chan cmdResponse),
	}

	result := z.addJobAndWait(nw, job)

	return result.exists, result.err
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
func (z *Zencached) extractVersionCmdValue(response []byte) (start, end int, err error) {

	start = -1
	end = len(response) - 2

	for i := len(response) - 1; i >= 0; i-- {
		if response[i] == whiteSpace {
			start = i + 1
			break
		}
	}

	if start == -1 {
		err = fmt.Errorf("no value found")
	}

	return
}

// Version - performs a version operation
func (z *Zencached) Version(routerHash []byte) ([]byte, error) {

	nw, _, err := z.GetConnectedNodeWorkers(routerHash, nil, nil)
	if err != nil {
		return nil, err
	}

	return z.baseVersion(nw)
}

// baseVersion - performs a get operation
func (z *Zencached) baseVersion(nw *nodeWorkers) ([]byte, error) {

	job := cmdJob{
		cmd:                  Version,
		beginResponseSet:     mcrVersionResponseSet,
		endResponseSet:       mcrVersionResponseSet,
		renderedCmd:          z.renderNoParameterCmd(Version),
		forceCacheMissMetric: true,
		response:             make(chan cmdResponse),
	}

	result := z.addJobAndWait(nw, job)

	if !result.exists || result.err != nil {
		return nil, result.err
	}

	start, end, err := z.extractVersionCmdValue([]byte(result.response))
	if err != nil {
		return nil, err
	}

	return result.response[start:end], nil
}
