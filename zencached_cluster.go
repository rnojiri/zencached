package zencached

import "context"

//
// Functions to distribute a key to all the cluster.
// author: rnojiri
//

// ClusterSet - performs an full set operation operation
func (z *Zencached) ClusterSet(ctx context.Context, path, key, value []byte, ttl uint64) ([]*OperationResult, []error) {

	return z.clusterStore(ctx, Set, path, key, value, ttl)
}

// ClusterAdd - performs an full add operation operation
func (z *Zencached) ClusterAdd(ctx context.Context, path, key, value []byte, ttl uint64) ([]*OperationResult, []error) {

	return z.clusterStore(ctx, Add, path, key, value, ttl)
}

// clusterStore - performs an full storage operation operation
func (z *Zencached) clusterStore(ctx context.Context, cmd MemcachedCommand, path, key, value []byte, ttl uint64) ([]*OperationResult, []error) {

	numNodes := len(z.nodeWorkerArray)

	results := make([]*OperationResult, numNodes)
	errors := make([]error, numNodes)

	if err := ValidateKey(path, key); err != nil {

		for i := 0; i < numNodes; i++ {
			results[i] = nil
			errors[i] = err
		}

		return results, errors
	}

	for i := 0; i < numNodes; i++ {

		nw, err := z.GetNodeWorkersByIndex(i)
		if err != nil {
			errors[i] = err
			continue
		}

		results[i], errors[i] = z.baseStore(ctx, nw, cmd, path, key, value, ttl)
	}

	return results, errors
}

// ClusterGet - returns a full replicated key stored in the cluster
func (z *Zencached) ClusterGet(ctx context.Context, path, key []byte) ([]*OperationResult, []error) {

	numNodes := len(z.nodeWorkerArray)

	results := make([]*OperationResult, numNodes)
	errors := make([]error, numNodes)

	if err := ValidateKey(path, key); err != nil {

		for i := 0; i < numNodes; i++ {
			results[i] = nil
			errors[i] = err
		}

		return results, errors
	}

	for i := 0; i < numNodes; i++ {

		nw, err := z.GetNodeWorkersByIndex(i)
		if err != nil {
			errors[i] = err
			continue
		}

		results[i], errors[i] = z.baseGet(ctx, nw, path, key)
	}

	return results, errors
}

// ClusterDelete - deletes a key from all cluster nodes
func (z *Zencached) ClusterDelete(ctx context.Context, path, key []byte) ([]*OperationResult, []error) {

	numNodes := len(z.nodeWorkerArray)

	results := make([]*OperationResult, numNodes)
	errors := make([]error, numNodes)

	if err := ValidateKey(path, key); err != nil {

		for i := 0; i < numNodes; i++ {
			results[i] = nil
			errors[i] = err
		}

		return results, errors
	}

	for i := 0; i < numNodes; i++ {

		nw, err := z.GetNodeWorkersByIndex(i)
		if err != nil {
			errors[i] = err
			continue
		}

		results[i], errors[i] = z.baseDelete(ctx, nw, path, key)
	}

	return results, errors
}
