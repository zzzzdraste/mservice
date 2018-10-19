# mservice

Scheduler microservice with Etcd discovery and Redis. The HTTP endpoint allows to push an order to the service, which will be stored in the ZSET in Redis with UnixTimestamp Due Date as a score value. The goroutine will regularly read through the ZSET and pull the orders that reached DD and move them to the Redis List

## Getting Started

The configuration is placed in the config.json file, edit it before running. The service will regularly connect and self-refresh the Etcd cluster details. If the cluster is down, the service will halt. The service is configured to self-register with Etcd with a TTL that's configurable in the config.json

### Prerequisites

No third-party libs are needed, for compilation of the source code, the golang should be installed and $GOPATH variable set

## License

This project is licensed under the GNU - see the [LICENSE.md](LICENSE.md) file for details

