package main

import (
	"fmt"
	"io/ioutil"
	"ms/common"
	customLog "ms/log"
	"ms/mserver"
	"net/http"
	"runtime"
	"time"
)

var (
	logger customLog.LevelLogger
	config common.Config
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	configData, err := ioutil.ReadFile("./config.json")
	if err != nil {
		panic(err)
	}
	err = common.Convert(configData, &config)
	if err != nil {
		panic(err)
	}

	logger = customLog.InitLogger(
		config.Logger["Trace"],
		config.Logger["Info"],
		config.Logger["Warning"],
		config.Logger["Error"])
}

func main() {
	defer recoverFromPanic()

	logger.Info.Println("starting microservice")
	logger.Info.Println("configuring Etcd cluster connection")
	cluster := initEtcdCluster()

	if cluster.GetNumberOfServers() == 0 {
		panic(fmt.Errorf("connection to Etcd cluster failed, no server is reachable"))
	}

	chEtcdReg := make(chan error)
	registerMS(cluster, chEtcdReg)

	logger.Info.Printf("starting the HTTP server for accepting incoming requests on %v:%v\n", config.HTTPServerHost, config.HTTPServerPort)
	httpServer := mserver.GetHTTPServer(
		config.HTTPServerHost,
		config.HTTPServerPort,
		time.Duration(config.HTTPServerReadTimeout)*time.Second,
		time.Duration(config.HTTPServerWriteTimeout)*time.Second)

	logger.Info.Println("registrating the HTTP server handler functions")
	logger.Trace.Println("register general endpoints")
	registerHTTPGeneralEndpoints(&httpServer)
	logger.Trace.Println("register Etcd endpoints")
	registerHTTPEtcdEndpoints(&httpServer, cluster)
	logger.Trace.Println("register service endpoints")
	registerHTTPServiceEndpoints(&httpServer)

	chHTTP := make(chan error)
	httpServer.RunHTTPServer(chHTTP)
	logger.Info.Println("HTTP server started, error channel created for critical errors propagation")

	chEtcd := make(chan error)
	cluster.UpdateMembersRoutine(config, chEtcd)
	logger.Info.Println("routine to update the Etcd cluster information started, error channel created for critical errors propagation")

	logger.Info.Println("routine scheduler service started")
	startSchedulerService()

	for {
		select {
		case clusterUpdateErr := <-chEtcd:
			panic(clusterUpdateErr)
		case serverStatusErr := <-chHTTP:
			panic(serverStatusErr)
		case etcdMSRegistrationErr := <-chEtcdReg:
			panic(etcdMSRegistrationErr)
		}
	}
}

// registerHTTPGeneralEndpoints registers the endpoints from the general section of the config
func registerHTTPGeneralEndpoints(httpServer *mserver.MServer) {
	for _, endpoint := range config.Endpoints.General {
		http.HandleFunc(endpoint, httpServer.HandlerMap[endpoint].(func(customLog.LevelLogger, *common.Config) http.HandlerFunc)(logger, &config))
	}
}

// registerHTTPEtcdEndpoints registers the endpoints from the general section of the config
func registerHTTPEtcdEndpoints(httpServer *mserver.MServer, cluster *common.Cluster) {
	for _, endpoint := range config.Endpoints.Etcd {
		http.HandleFunc(endpoint,
			httpServer.HandlerMap[endpoint].(func(*common.Cluster, customLog.LevelLogger) http.HandlerFunc)(cluster, logger))
	}
}

// registerHTTPServiceEndpoints registers the endpoints from the general section of the config
func registerHTTPServiceEndpoints(httpServer *mserver.MServer) {
	for _, endpoint := range config.Endpoints.Service {
		http.HandleFunc(endpoint,
			httpServer.HandlerMap[endpoint].(func(customLog.LevelLogger, *common.Config) http.HandlerFunc)(logger, &config))
	}
}

// initEtcdCluster populates the cluster from the configuration file, creates servers and bundles them into the cluster
// the function would then return a pointer to the cluster object to be used for subsequent calls
func initEtcdCluster() *common.Cluster {
	cluster := common.NewCluster()

	for _, serverConfig := range config.Etcd.Cluster.EtcdServers {
		server := common.NewServer(serverConfig.ClientEndpoints, serverConfig.ServerEndpoints)
		err := server.Connect(config)
		if err != nil {
			logger.Error.Printf("connection to the Etcd server (client endpoints:%v, server endpoints: %v) failed: %v\n",
				serverConfig.ClientEndpoints, serverConfig.ServerEndpoints, err)
		} else {
			cluster.AddEndpoint(server)
		}
	}
	return &cluster
}

// registerMS is used to register microservice with Etcd
func registerMS(c *common.Cluster, ch chan error) {
	logger.Info.Println("microservice registration with Etcd started")

	sleepTime := time.Duration(config.TTL - config.TTL/10)
	url := "http://" + config.HTTPServerHost + ":" + config.HTTPServerPort

	go func() {
		logger.Trace.Printf("the microservice endpoint will be set in Etcd %v=%v with TTL %v\n", config.Name, url, config.TTL)
		for {
			server, _ := c.GetRandomServer()
			_, err := server.Set(config.Name, url, config.TTL)
			if config.TTL < 0 {
				logger.Trace.Printf("TTL value (%v) is negative, the key will not expire, no refresh is needed", config.TTL)
				break
			}
			if err != nil {
				ch <- err
			}

			time.Sleep(sleepTime * time.Second)
		}
	}()
}

func startSchedulerService() {
	err := mserver.StartScheduler(&config, logger)
	if err != nil {
		panic(err)
	}
}

// recoverFromPanic allows to gracefully recover (if needed) from the panic state and not to fail with trace
// use it to overwrite the default behavior
func recoverFromPanic() {
	if r := recover(); r != nil {
		logger.Error.Printf("unrecoverable state: %v\nSHUTTING DOWN..............\n", r)
	}
}
