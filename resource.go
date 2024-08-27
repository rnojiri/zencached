package zencached

import "sync/atomic"

type resourceChannel struct {
	channel      chan struct{}
	numAvailable atomic.Uint32
}

func newResource(numResources uint32) *resourceChannel {

	r := make(chan struct{}, numResources)

	for i := uint32(0); i < numResources; i++ {
		r <- struct{}{}
	}

	return &resourceChannel{
		channel:      r,
		numAvailable: atomic.Uint32{},
	}
}

func (r *resourceChannel) take() bool {

	select {
	case <-r.channel:
		r.numAvailable.Add(^uint32(0))
		return true
	default:
		return false
	}
}

func (r *resourceChannel) put() {

	r.channel <- struct{}{}
	r.numAvailable.Add(1)
}

func (r *resourceChannel) terminate() {

	close(r.channel)
	r.numAvailable.Store(0)
}

func (r *resourceChannel) numAvailableResources() uint32 {

	return r.numAvailable.Load()
}
