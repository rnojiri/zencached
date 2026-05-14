# Documentation Index

Welcome to the Zencached documentation. This index will help you find what you need.

## Quick Links

- **[Getting Started](../README.md#quick-start)** - Get up and running in 5 minutes
- **[API Reference](./API.md)** - Complete API documentation
- **[Examples](./EXAMPLES.md)** - Real-world code examples
- **[Troubleshooting](./TROUBLESHOOTING.md)** - Common issues and solutions

## Documentation Structure

### For New Users

Start here if you're new to Zencached:

1. [README.md](../README.md) - Project overview and features
2. [README.md - Quick Start](../README.md#quick-start) - Simple hello-world example
3. [EXAMPLES.md](./EXAMPLES.md) - More practical examples
4. [CONFIGURATION.md](./CONFIGURATION.md) - Setup your client

### For Developers

If you're building with Zencached:

1. [API.md](./API.md) - Complete API reference
2. [EXAMPLES.md](./EXAMPLES.md) - Code examples for different scenarios
3. [CONFIGURATION.md](./CONFIGURATION.md) - Advanced configuration options
4. [ERROR_TYPES.md](./ERROR_TYPES.md) - Error handling

### For Operations/DevOps

If you're deploying and monitoring:

1. [CONFIGURATION.md](./CONFIGURATION.md) - Performance tuning
2. [METRICS.md](./METRICS.md) - Metrics collection and monitoring
3. [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) - Problem solving
4. [ARCHITECTURE.md](./ARCHITECTURE.md) - System internals

### For Contributors

If you want to contribute:

1. [CONTRIBUTING.md](./CONTRIBUTING.md) - Contribution guidelines
2. [ARCHITECTURE.md](./ARCHITECTURE.md) - System design
3. [README.md](../README.md) - Project overview

## Documentation Files

### Core Documentation

| File | Purpose | Audience |
|------|---------|----------|
| [README.md](../README.md) | Project overview, features, quick start | Everyone |
| [API.md](./API.md) | Complete API reference | Developers |
| [EXAMPLES.md](./EXAMPLES.md) | Practical code examples | Developers |
| [CONFIGURATION.md](./CONFIGURATION.md) | Setup and configuration | Developers, DevOps |
| [ARCHITECTURE.md](./ARCHITECTURE.md) | System design and internals | Contributors, DevOps |

### Reference Documentation

| File | Purpose | Audience |
|------|---------|----------|
| [ERROR_TYPES.md](./ERROR_TYPES.md) | Error handling reference | Developers |
| [METRICS.md](./METRICS.md) | Metrics collection reference | DevOps, Developers |
| [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) | Common issues and solutions | Everyone |
| [CONTRIBUTING.md](./CONTRIBUTING.md) | Contribution guidelines | Contributors |

## Common Tasks

### I want to...

#### Get started with Zencached
1. Read [README.md](../README.md) overview
2. Follow [README.md - Quick Start](../README.md#quick-start)
3. Check [EXAMPLES.md](./EXAMPLES.md#basic-operations)

#### Configure compression
1. See [CONFIGURATION.md - Compression Settings](./CONFIGURATION.md#compression-settings)
2. Review [EXAMPLES.md#compression](./EXAMPLES.md#compression)
3. Check [ARCHITECTURE.md - Compression Pipeline](./ARCHITECTURE.md#compression-pipeline)

#### Set up metrics collection
1. Check [METRICS.md](./METRICS.md)
2. See [EXAMPLES.md#metrics-collection](./EXAMPLES.md#metrics-collection)
3. Review [CONFIGURATION.md - Metrics Configuration](./CONFIGURATION.md#metrics-configuration)

#### Handle errors properly
1. See [ERROR_TYPES.md](./ERROR_TYPES.md)
2. Check [EXAMPLES.md#error-handling](./EXAMPLES.md#error-handling)
3. Review [ERROR_TYPES.md - Error Handling Strategies](./ERROR_TYPES.md#error-handling-strategies)

#### Set up dynamic node discovery
1. Check [CONFIGURATION.md - Dynamic Node Discovery](./CONFIGURATION.md#dynamic-node-discovery)
2. See [EXAMPLES.md#dynamic-node-discovery](./EXAMPLES.md#dynamic-node-discovery)
3. Review [ARCHITECTURE.md - Node Rebalancing](./ARCHITECTURE.md#node-rebalancing)

#### Troubleshoot connection issues
1. See [TROUBLESHOOTING.md - Connection Issues](./TROUBLESHOOTING.md#connection-issues)
2. Check [ERROR_TYPES.md - Connection Errors](./ERROR_TYPES.md#connection-errors)

#### Optimize for performance
1. Read [CONFIGURATION.md - Performance Tuning](./CONFIGURATION.md#performance-tuning)
2. Check [ARCHITECTURE.md - Performance Characteristics](./ARCHITECTURE.md#performance-characteristics)
3. See [TROUBLESHOOTING.md - Performance Issues](./TROUBLESHOOTING.md#performance-issues)

#### Contribute code
1. Read [CONTRIBUTING.md](./CONTRIBUTING.md)
2. Understand the [ARCHITECTURE.md](./ARCHITECTURE.md)

## Search Tips

### By Topic

- **Cluster operations**: [API.md](./API.md#cluster-operations), [EXAMPLES.md](./EXAMPLES.md#cluster-operations)
- **Compression**: [CONFIGURATION.md](./CONFIGURATION.md#compression-settings), [EXAMPLES.md](./EXAMPLES.md#compression), [ARCHITECTURE.md](./ARCHITECTURE.md#compression-pipeline)
- **Connection management**: [ARCHITECTURE.md](./ARCHITECTURE.md#connection-management), [CONFIGURATION.md](./CONFIGURATION.md#connection-settings)
- **Context/timeout**: [API.md](./API.md#common-patterns), [EXAMPLES.md](./EXAMPLES.md#context-usage)
- **Error handling**: [ERROR_TYPES.md](./ERROR_TYPES.md), [EXAMPLES.md](./EXAMPLES.md#error-handling)
- **Metrics**: [METRICS.md](./METRICS.md), [EXAMPLES.md](./EXAMPLES.md#metrics-collection)
- **Node discovery**: [CONFIGURATION.md](./CONFIGURATION.md#dynamic-node-discovery), [EXAMPLES.md](./EXAMPLES.md#dynamic-node-discovery)
- **Performance**: [CONFIGURATION.md](./CONFIGURATION.md#performance-tuning), [TROUBLESHOOTING.md](./TROUBLESHOOTING.md#performance-issues)

### By Operation

- **Set**: [API.md - Set](./API.md#set), [EXAMPLES.md](./EXAMPLES.md#basic-operations)
- **Get**: [API.md - Get](./API.md#get), [EXAMPLES.md](./EXAMPLES.md#basic-operations)
- **Delete**: [API.md - Delete](./API.md#delete), [EXAMPLES.md](./EXAMPLES.md#basic-operations)
- **Add**: [API.md - Add](./API.md#add), [EXAMPLES.md](./EXAMPLES.md#basic-operations)
- **Cluster operations**: [API.md - Cluster Operations](./API.md#cluster-operations), [EXAMPLES.md](./EXAMPLES.md#cluster-operations)
- **Node management**: [API.md - Node Management](./API.md#node-management), [ARCHITECTURE.md](./ARCHITECTURE.md#node-rebalancing)

### By Error Type

Check [ERROR_TYPES.md](./ERROR_TYPES.md) for all error types and their solutions.

Common errors:
- Connection errors: [ERROR_TYPES.md - Connection Errors](./ERROR_TYPES.md#connection-errors)
- Key errors: [ERROR_TYPES.md - Key/Value Errors](./ERROR_TYPES.md#keyvalue-errors)
- I/O errors: [ERROR_TYPES.md - I/O Errors](./ERROR_TYPES.md#io-errors)
- Compression errors: [ERROR_TYPES.md - Compression Errors](./ERROR_TYPES.md#compression-errors)

## Learning Path

### Beginner
1. [README.md](../README.md) - Overview
2. [README.md - Quick Start](../README.md#quick-start) - First code
3. [EXAMPLES.md#basic-operations](./EXAMPLES.md#basic-operations) - More examples

### Intermediate
1. [API.md](./API.md) - Full API reference
2. [CONFIGURATION.md](./CONFIGURATION.md) - Configuration options
3. [EXAMPLES.md](./EXAMPLES.md) - Various scenarios
4. [ERROR_TYPES.md](./ERROR_TYPES.md) - Error handling

### Advanced
1. [ARCHITECTURE.md](./ARCHITECTURE.md) - System design
2. [METRICS.md](./METRICS.md) - Metrics collection
3. [CONFIGURATION.md#performance-tuning](./CONFIGURATION.md#performance-tuning) - Optimization
4. [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) - Debugging

### Expert/Contributing
1. [CONTRIBUTING.md](./CONTRIBUTING.md) - Contribution guidelines
2. [ARCHITECTURE.md](./ARCHITECTURE.md) - Deep dive
3. Review codebase with architectural knowledge

## Quick Reference

### Key Types

- `Zencached` - Main client
- `Configuration` - Configuration structure
- `Node` - Node definition
- `OperationResult` - Operation result
- `ZError` - Error interface
- `ZencachedMetricsCollector` - Metrics interface

See [API.md - Data Types](./API.md#data-types) for complete reference.

### Key Operations

- `Set()` - Store a value
- `Get()` - Retrieve a value
- `Add()` - Store if not exists
- `Delete()` - Remove a value
- `ClusterSet()` - Store on all nodes
- `ClusterGet()` - Get from all nodes
- `Rebalance()` - Rebalance nodes
- `Shutdown()` - Cleanup

See [API.md - Basic Operations](./API.md#basic-operations) for details.

## Resources

### External Links

- [Memcached Protocol Documentation](https://github.com/memcached/memcached/blob/master/doc/protocol.txt)
- [Go Context Package](https://pkg.go.dev/context)
- [Go Modules](https://golang.org/ref/mod)

### Related Projects

- [logh](https://github.com/rnojiri/logh) - Logging library used by Zencached
- [klauspost/compress](https://github.com/klauspost/compress) - Compression library

## Getting Help

1. **Check documentation** - Start with relevant docs above
2. **Search examples** - Look for similar use cases in [EXAMPLES.md](./EXAMPLES.md)
3. **Troubleshoot** - See [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)
4. **Report issues** - Open GitHub issue with details

## Documentation Feedback

Found an issue or have suggestions for documentation? Please:

1. Open an issue on GitHub
2. Provide specific location and feedback
3. Suggest improvements

Your feedback helps make the documentation better for everyone!
