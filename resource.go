package zencached

type resourceChannel struct {
	channel chan struct{}
}

func newResource(numResources uint32) *resourceChannel {

	r := make(chan struct{}, numResources)

	for i := uint32(0); i < numResources; i++ {
		r <- struct{}{}
	}

	return &resourceChannel{
		channel: r,
	}
}

func (r *resourceChannel) take() bool {

	select {
	case <-r.channel:
		return true
	default:
		return false
	}
}

func (r *resourceChannel) put() {

	r.channel <- struct{}{}
}

func (r *resourceChannel) terminate() {

	close(r.channel)
}

func (r *resourceChannel) numAvailableResources() int {

	return len(r.channel)
}
