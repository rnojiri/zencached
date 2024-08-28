package zencached

//
// Functions to distribute a key to all the cluster.
// author: rnojiri
//

// ClusterSet - performs an full set operation operation
func (z *Zencached) ClusterSet(path, key, value []byte, ttl uint64) ([]*OperationResult, []error) {

	return z.clusterStore(Set, path, key, value, ttl)
}

// ClusterAdd - performs an full add operation operation
func (z *Zencached) ClusterAdd(path, key, value []byte, ttl uint64) ([]*OperationResult, []error) {

	return z.clusterStore(Add, path, key, value, ttl)
}

// clusterStore - performs an full storage operation operation
func (z *Zencached) clusterStore(cmd MemcachedCommand, path, key, value []byte, ttl uint64) ([]*OperationResult, []error) {

	numNodes := len(z.nodeWorkerArray)

	results := make([]*OperationResult, numNodes)
	errors := make([]error, numNodes)

	for i := 0; i < numNodes; i++ {

		nw, err := z.GetNodeWorkersByIndex(i)
		if err != nil {
			errors[i] = err
			continue
		}

		results[i], errors[i] = z.baseStore(nw, cmd, path, key, value, ttl)
	}

	return results, errors
}

// ClusterGet - returns a full replicated key stored in the cluster
func (z *Zencached) ClusterGet(path, key []byte) ([]*OperationResult, []error) {

	numNodes := len(z.nodeWorkerArray)

	results := make([]*OperationResult, numNodes)
	errors := make([]error, numNodes)

	for i := 0; i < numNodes; i++ {

		nw, err := z.GetNodeWorkersByIndex(i)
		if err != nil {
			errors[i] = err
			continue
		}

		results[i], errors[i] = z.baseGet(nw, path, key)
	}

	return results, errors
}

// ClusterDelete - deletes a key from all cluster nodes
func (z *Zencached) ClusterDelete(path, key []byte) ([]*OperationResult, []error) {

	numNodes := len(z.nodeWorkerArray)

	results := make([]*OperationResult, numNodes)
	errors := make([]error, numNodes)

	for i := 0; i < numNodes; i++ {

		nw, err := z.GetNodeWorkersByIndex(i)
		if err != nil {
			errors[i] = err
			continue
		}

		results[i], errors[i] = z.baseDelete(nw, path, key)
	}

	return results, errors
}
