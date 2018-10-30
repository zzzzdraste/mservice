# mservice

Scheduler microservice with Etcd discovery and Redis. The HTTP endpoint allows to push an order to the service, which will be stored in the ZSET in Redis with UnixTimestamp Due Date as a score value. The goroutine will regularly read through the ZSET and pull the orders that reached DD and move them to the Redis List

## Getting Started

The configuration is placed in the config.json file, edit it before running. The service will regularly connect and self-refresh the Etcd cluster details. If the cluster is down, the service will halt. The service is configured to self-register with Etcd with a TTL that's configurable in the config.json

### Prerequisites

No third-party libs are needed, for compilation of the source code, the golang should be installed and $GOPATH variable set

## License

This project is licensed under the GNU - see the [LICENSE.md](LICENSE.md) file for details

## Performance testing
With only error logging enabled (the rest > /dev/null)
```
ab -n 500000 -c 10 -k 'http://localhost:8080/order/test?dd=1540907139&order_id=test_5'
This is ApacheBench, Version 2.3 <$Revision: 1826891 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking localhost (be patient)
Completed 50000 requests
Completed 100000 requests
Completed 150000 requests
Completed 200000 requests
Completed 250000 requests
Completed 300000 requests
Completed 350000 requests
Completed 400000 requests
Completed 450000 requests
Completed 500000 requests
Finished 500000 requests


Server Software:        
Server Hostname:        localhost
Server Port:            8080

Document Path:          /order/test?dd=1540907139&order_id=test_5
Document Length:        95 bytes

Concurrency Level:      10
Time taken for tests:   58.168 seconds
Complete requests:      500000
Failed requests:        0
Keep-Alive requests:    500000
Total transferred:      118000000 bytes
HTML transferred:       47500000 bytes
Requests per second:    8595.81 [#/sec] (mean)
Time per request:       1.163 [ms] (mean)
Time per request:       0.116 [ms] (mean, across all concurrent requests)
Transfer rate:          1981.07 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        0    0   0.0      0       1
Processing:     0    1   1.3      1      56
Waiting:        0    1   1.3      1      56
Total:          0    1   1.3      1      56

Percentage of the requests served within a certain time (ms)
  50%      1
  66%      1
  75%      1
  80%      1
  90%      2
  95%      2
  98%      4
  99%      7
 100%     56 (longest request)
```


