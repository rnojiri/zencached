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

	// response set
	mcrStoredResponseSet      memcachedResponseSet = memcachedResponseSet{mcrStored, mcrNotStored}
	mcrGetCheckResponseSet    memcachedResponseSet = memcachedResponseSet{mcrEnd}
	mcrGetCheckEndResponseSet memcachedResponseSet = memcachedResponseSet{mcrValue, mcrEnd}
	mcrDeletedResponseSet     memcachedResponseSet = memcachedResponseSet{mcrDeleted, mcrNotFound}

	ErrMemcachedInvalidResponse error = errors.New("invalid memcached command response received")
	ErrMemcachedNoResponse      error = errors.New("no response from memcached")
	ErrMaxCommandRetriesReached error = errors.New("maximum number of command retries reached")
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

	// Delete - return a key if it exists or not
	Delete MemcachedCommand = MemcachedCommand("delete")
)

// checkResponse - checks the memcached response
func (z *Zencached) checkResponse(
	telnetConn *Telnet,
	checkReadSet, checkResponseSet memcachedResponseSet,
) (bool, []byte, error) {

	response, err := telnetConn.Read(checkReadSet)
	if err != nil {
		return false, nil, err
	}

	if len(response) == 0 {
		return false, nil, ErrMemcachedNoResponse
	}

	if !bytes.HasPrefix(response, checkResponseSet[0]) {
		if !bytes.Contains(response, checkResponseSet[1]) {
			return false, nil, fmt.Errorf("%w: %s", ErrMemcachedInvalidResponse, string(response))
		}

		return false, response, nil
	}

	return true, response, nil
}

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

// store - internal stores
func (z *Zencached) store(cmd MemcachedCommand, routerHash, path, key, value []byte, ttl uint64) (bool, error) {

	telnetConn, index, err := z.GetTelnetConnection(routerHash, path, key)
	if err != nil {
		return false, err
	}

	defer z.ReturnTelnetConnection(telnetConn, index)

	return z.baseStore(telnetConn, cmd, routerHash, path, key, value, ttl)
}

// baseStore - base storage function
func (z *Zencached) baseStore(telnetConn *Telnet, cmd MemcachedCommand, routerHash, path, key, value []byte, ttl uint64) (bool, error) {

	stored, _, err := z.sendCommand(
		telnetConn,
		cmd,
		mcrStoredResponseSet, mcrStoredResponseSet,
		z.renderStoreCmd(cmd, path, key, value, ttl),
		path, key,
		0,
		true,
	)

	return stored, err
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

	telnetConn, index, err := z.GetTelnetConnection(routerHash, path, key)
	if err != nil {
		return nil, false, err
	}

	defer z.ReturnTelnetConnection(telnetConn, index)

	return z.baseGet(telnetConn, path, key)
}

// baseGet - the base get operation
func (z *Zencached) baseGet(telnetConn *Telnet, path, key []byte) ([]byte, bool, error) {

	exists, response, err := z.sendCommand(
		telnetConn,
		Get,
		mcrGetCheckResponseSet, mcrGetCheckEndResponseSet,
		z.renderKeyOnlyCmd(Get, path, key),
		path, key,
		0,
		false,
	)
	if !exists || err != nil {
		return nil, false, err
	}

	start, end, err := z.extractValue([]byte(response))
	if err != nil {
		return nil, false, err
	}

	return response[start:end], true, nil
}

// extractValue - extracts a value from the response
func (z *Zencached) extractValue(response []byte) (start, end int, err error) {

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

// Delete - performs a delete operation
func (z *Zencached) Delete(routerHash, path, key []byte) (bool, error) {

	telnetConn, index, err := z.GetTelnetConnection(routerHash, path, key)
	if err != nil {
		return false, err
	}

	defer z.ReturnTelnetConnection(telnetConn, index)

	return z.baseDelete(telnetConn, path, key)
}

// baseDelete - base delete operation
func (z *Zencached) baseDelete(telnetConn *Telnet, path, key []byte) (bool, error) {

	exists, _, err := z.sendCommand(
		telnetConn,
		Delete,
		mcrDeletedResponseSet, mcrDeletedResponseSet,
		z.renderKeyOnlyCmd(Delete, path, key),
		path, key,
		0,
		false,
	)

	return exists, err
}

func (z *Zencached) sendAndReadResponse(
	telnetConn *Telnet,
	beginResponseSet, endResponseSet memcachedResponseSet,
	renderedCmd []byte,
) (exists bool, response []byte, err error) {

	err = telnetConn.Send(renderedCmd)
	if err == nil {
		exists, response, err = z.checkResponse(telnetConn, beginResponseSet, endResponseSet)
	}

	return
}

func (z *Zencached) sendCommand(
	telnetConn *Telnet,
	cmd MemcachedCommand,
	beginResponseSet, endResponseSet memcachedResponseSet,
	renderedCmd, path, key []byte,
	numRetry int,
	forceCacheMissMetric bool,
) (exists bool, response []byte, err error) {

	if z.configuration.MetricsCollector == nil {

		exists, response, err = z.sendAndReadResponse(telnetConn, beginResponseSet, endResponseSet, renderedCmd)

	} else {

		z.configuration.MetricsCollector.CommandExecution(telnetConn.GetHost(), cmd, path, key)

		start := time.Now()

		exists, response, err = z.sendAndReadResponse(telnetConn, beginResponseSet, endResponseSet, renderedCmd)

		elapsedTime := time.Since(start)

		z.configuration.MetricsCollector.CommandExecutionElapsedTime(
			telnetConn.GetHost(),
			cmd,
			path, key,
			elapsedTime.Milliseconds(),
		)

		if exists && !forceCacheMissMetric {
			z.configuration.MetricsCollector.CacheHitEvent(telnetConn.GetHost(), cmd, path, key)
		} else {
			z.configuration.MetricsCollector.CacheMissEvent(telnetConn.GetHost(), cmd, path, key)
		}

		if err != nil {

			z.configuration.MetricsCollector.CommandExecutionError(
				telnetConn.GetHost(),
				cmd,
				path, key,
				err,
			)
		}
	}

	if errors.Is(err, ErrMaxReconnectionsReached) || errors.Is(err, ErrMemcachedNoResponse) {

		if numRetry < z.configuration.NumCommandRetries {
			return z.sendCommand(telnetConn, cmd, beginResponseSet, endResponseSet, renderedCmd, path, key, numRetry+1, forceCacheMissMetric)
		}

		if z.configuration.RebalanceOnDisconnection {
			go z.Rebalance()
		}

		err = ErrMaxCommandRetriesReached
	}

	return
}
