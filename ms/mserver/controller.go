package mserver

import (
	"encoding/json"
	"expvar"
	"fmt"
	"html/template"
	"log"
	"ms/common"
	myLog "ms/log"
	"ms/redis"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	helpTemplate            *template.Template
	numberOfRequests        = expvar.NewInt("number_of_requests")
	numberOfErrors          = expvar.NewInt("number_of_errors")
	numberOfSuccessResponse = expvar.NewInt("number_of_success_responses")
)

func init() {
	helpText := `<html>
<head>
<h1>HELP on the MS available API calls</h1>
<h3>Individual endpoints could be disabled in the config.json file, endpoints section. The endpoints are logically divided into three groups</h3>
</head>
<body>
<table>
<tr><td colspan="2" align="center"><b>general</b></td></tr>
<tr><td>/help/{transaction_id}></td><td>shows this help message</td></tr>
<tr><td>/version/{transaction_id}</td><td>respond sent with the current version of the MS</td></tr>
<tr><td colspan="2" align="center"><b>etcd</b></td></tr>
<tr><td>/etcdMemebers/{transaction_id}</td><td>lists the current members of the Etcd cluster, the list is autoupdated regularly, including the endpoints</td></tr>
<tr><td>/etcdVersion/{trasaction_id}</td><td>lists the current etcd cluster and server versions</td></tr>
<tr><td>/etcdSet/{transaction_id}?key=&value=</td><td>allows to set a new key-value pair in the Etcd cluster (the operation is idempotent), returns the confirmation  if success, or failure message otherwise</td></tr>
<tr><td>/etcdGet/{transaction_id}?key=</td><td>request to Etcd server to get the value for the given key</td></tr>
<tr><td colspan="2" align="center"><b>service</b></td></tr>
<tr><td>/order/{transaction_id}?dd=&order_id=</td><td>pass a new transaction into the service</td></tr>
<tr><td colspan="2" align="center"><b>metrics info</b></td></tr>
<tr><td>/debug/vars</td><td>shows processing metrics and custom expvar metrics</td></tr>
</table>
</body>
</html>`
	helpTemplate = template.New("Help Template")
	helpTemplate.Parse(helpText)
}

// GetHTTPServer constructs the HTTP Server object (mServer) and presets the map of available endpoints
func GetHTTPServer(ip, port string, readTimeout, writeTimeout time.Duration) MServer {
	httpServer := MServer{
		HandlerMap: map[string]interface{}{
			"/etcdMembers/": EtcdListCurrentMembers,
			"/etcdVersion/": EtcdVersion,
			"/etcdSet/":     EtcdSetKeyValuePair,
			"/etcdGet/":     EtcdGetValueForTheKey,
			"/help/":        ShowHelpInfo,
			"/version/":     Version,
			"/order/":       NewOrderService,
		},
	}
	httpServer.Server = &http.Server{
		Addr:              ip + ":" + port,
		ReadHeaderTimeout: readTimeout,
		WriteTimeout:      writeTimeout,
		ErrorLog:          log.New(os.Stdout, "http server: ", log.LstdFlags),
	}
	return httpServer
}

// RunHTTPServer runs a goroutine with HTTP ListenAndServe function and passes a channel for error communication
func (s *MServer) RunHTTPServer(ch chan error) {
	go func(ch chan error) {
		err := s.Server.ListenAndServe()
		if err != nil {
			ch <- err
		}
	}(ch)
}

// MServer struct is used to configure and start the HTTP Server with handlers predifined
type MServer struct {
	// Server is a pointer to the http.Server used to start the listener for external systems
	Server *http.Server
	// HandlerMap is used to register all the endpoint Handers (http.HandlerFunc). Based on the user configuration
	// some of them may be ignored and no endpoint will be served. The map is initialized with the cardcoaded list of
	// the known endpoints and functions; user config can be used to server all or some
	HandlerMap map[string]interface{}
}

func generateErrorPayload(id string, errorCode int, err error) []byte {
	numberOfErrors.Add(1)

	errorPayload := common.ResponsePayload{
		TransactionID: id,
		Status:        errorCode,
		Message:       err.Error(),
	}
	output, _ := json.MarshalIndent(errorPayload, "", "    ")
	return output
}

func generateReponsePayload(id string, reponseCode int, msg string) []byte {
	numberOfSuccessResponse.Add(1)

	reponsePayload := common.ResponsePayload{
		TransactionID: id,
		Status:        reponseCode,
		Message:       msg,
	}

	output, _ := json.MarshalIndent(reponsePayload, "", "    ")
	return output
}

// parseTransactionID is a function to substring the URL and lookup for the transaction ID portion
// returns N/A if no ID present
func parseTransactionID(url string) string {
	numberOfRequests.Add(1)

	index := strings.LastIndex(url, "/")
	if index == len(url)-1 {
		return "N/A"
	}
	return url[index+1:]

}

// NewOrderService provides the endpoint HandlerFunc to process the request for a new order
// This is a service endpoint for actual MS processing logic
func NewOrderService(logger myLog.LevelLogger, config *common.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := r.URL
		transID := parseTransactionID(url.Path)

		dd := url.Query().Get("dd")
		// dd should be an integer/number representing unixtimestamp
		_, convErr := strconv.Atoi(dd)
		if convErr != nil {
			errMsg := fmt.Errorf("error converting the dd=%v param from the query to int value: %v", dd, convErr)
			output := generateErrorPayload(transID, http.StatusBadRequest, errMsg)
			w.Write(output)
			logger.Error.Printf("http server: %v: error converting the dd=%v param from the query to int value: %v\n", transID, dd, convErr)
			return
		}

		orderID := url.Query().Get("order_id")
		logger.Info.Printf("http server: %v: request received for the /order/ endpoint\n", transID)
		logger.Trace.Printf("http server: %v: dd=%v and order_id value=%v\n", transID, dd, orderID)

		if orderID == "" {
			errStr := fmt.Errorf("no order ID is passed in the request: %v", url.Query())
			output := generateErrorPayload(transID, http.StatusBadRequest, errStr)
			w.Write(output)
			logger.Error.Printf("http server: %v: order ID is missing in the request, should be passed as a query attrib ?order_id=\n", transID)
			return
		}

		logger.Trace.Printf("attempting to connect to the Redis server %v:%v\n", config.Service.RedisHost, config.Service.RedisPort)
		redisClient, redisErr := redis.Connect(config.Service.RedisHost, config.Service.RedisPort)
		if redisErr != nil {
			redisErrStr := fmt.Errorf("http server: %v: Redis client is unreachable: %v", transID, redisErr)
			output := generateErrorPayload(transID, http.StatusFailedDependency, redisErrStr)
			w.Write(output)
			logger.Error.Printf("http server: %v: connection to Redis server failed: %v\n", transID, redisErr)
			return
		}

		defer redisClient.Conn.Close()

		logger.Trace.Printf("http server: %v: constructing a ZADD command to redis server for orderID=%v and DD=%v\n", transID, orderID, dd)
		// we make a call to the Redis Scheduler (scheduler.go file)
		err := StoreNewOrderInRedis(config.Service.OrderSetName, dd, orderID, redisClient, logger)
		if err != nil {
			output := generateErrorPayload(transID, http.StatusInternalServerError, err)
			w.Write(output)
			logger.Error.Printf("http server: %v: error sending command to the Redis server: %v\n", transID, err)
		}

		redisClient.Conn.Close()

		// sending the output
		output := generateReponsePayload(transID, http.StatusCreated, "Order successfully created")
		logger.Trace.Printf("http server: sending 201 Created response to the caller\n")
		w.Write(output)
	}
}

// ShowHelpInfo is used to display the help information on the API available form the MS and the common usage
// TODO should we use godoc generated HELP instead upon the request?
func ShowHelpInfo(logger myLog.LevelLogger, config *common.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		transID := parseTransactionID(r.URL.Path)
		logger.Info.Printf("http server: %v: /help request received from %v\n", transID, r.RemoteAddr)
		helpTemplate.Execute(w, nil)
		logger.Info.Printf("http server: %v: help information was sent back to the caller %v\n", transID, r.RemoteAddr)
	}
}

// Version is a handler function for the /version call to the service, sends back the JSON with current version
func Version(logger myLog.LevelLogger, config *common.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		transID := parseTransactionID(r.URL.Path)
		logger.Info.Printf("http server: %v: /version request received from %v\n", transID, r.RemoteAddr)
		w.Write([]byte("{ \"version\": \"" + config.Version + "\" }"))
		logger.Info.Printf("http server: %v: version information was sent back to the caller %v\n", transID, r.RemoteAddr)
	}
}

// EtcdVersion is a handler function to display the version output to the called in JSON format
func EtcdVersion(c *common.Cluster, logger myLog.LevelLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		transID := parseTransactionID(r.URL.Path)
		var output []byte
		logger.Info.Printf("http server: %v: /etcdVersion request received from %v\n", transID, r.RemoteAddr)

		server, err := c.GetRandomServer()
		if err != nil {
			output = generateErrorPayload(transID, http.StatusInternalServerError, err)
			w.Write(output)
			logger.Error.Printf("http server: %v: %v\n", transID, err)
			return
		}

		logger.Trace.Printf("http server: %v: %v:%v selected for request processing\n", transID, server.ID, server.Name)
		data, err := server.Version()
		logger.Trace.Printf("http server: %v: []byte received from the Etcd server: %v\n", transID, string(data[:]))
		if err != nil {
			output = generateErrorPayload(transID, http.StatusInternalServerError, err)
			logger.Error.Printf("http server: %v: error processing request /etcdVersion: %v\n", transID, err)
			w.Write(output)
			return
		}
		var vObj common.EtcdVersionPayload

		err = common.Convert(data, &vObj)
		if err != nil {
			output = generateErrorPayload(transID, http.StatusInternalServerError, err)
			logger.Error.Printf("http server: %v: error convering Etcd server output into object: %v\n", transID, err)
			w.Write(output)
			return
		}

		output, err = json.MarshalIndent(vObj, "", "    ")
		if err != nil {
			output = generateErrorPayload(transID, http.StatusInternalServerError, err)
			logger.Error.Printf("http server: %v: error marshalling object into JSON: %v\n", transID, err)
		}

		w.Write(output)
		logger.Info.Printf("http server: %v: response: %v was sent to %v client\n", transID, string(output[:]), r.RemoteAddr)
	}
}

// EtcdListCurrentMembers is a Handler that sends back the list of current Etcd members
func EtcdListCurrentMembers(c *common.Cluster, logger myLog.LevelLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		transID := parseTransactionID(r.URL.Path)
		logger.Info.Printf("http server: %v: /etcdListMembers request received from %v\n", transID, r.RemoteAddr)
		output, err := json.MarshalIndent(c.GetServersList(), "", "    ")
		if err != nil {
			output = generateErrorPayload(transID, http.StatusInternalServerError, err)
			logger.Error.Printf("http server: %v: error getting the list of Etcd members: %v\n", transID, err)
		}
		w.Write(output)
		logger.Info.Printf("http server: %v: response was sent to %v client: %v\n", transID, string(output[:]), r.RemoteAddr)
	}
}

// EtcdGetValueForTheKey runs a query to the Etcd cluster to retreive a value for a given key
func EtcdGetValueForTheKey(c *common.Cluster, logger myLog.LevelLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		transID := parseTransactionID(r.URL.Path)
		var wait bool
		var output []byte

		query := r.URL.Query()
		key := query.Get("key")
		waitKey := query.Get("wait")
		if waitKey != "" {
			var errConv error
			wait, errConv = strconv.ParseBool(waitKey)
			if errConv != nil {
				output = generateErrorPayload(transID, http.StatusBadRequest,
					fmt.Errorf("the wait option in the request has non-bool value: %v", errConv))
				w.Write(output)
				logger.Error.Printf("http server: %v: the wait option in the request %v was not a bool value, should be true|false: %v",
					transID, query, errConv)
				return
			}
		}

		logger.Info.Printf("http server: %v: request /etcdGet received to get the value for the \"%v\" key from Etcd cluster, the \"wait\" is set to %v",
			transID, key, waitKey)
		if key == "" {
			output = generateErrorPayload(transID, http.StatusBadRequest,
				fmt.Errorf("the query %v contains empty value for the key or the key param is missing", r.URL))
			logger.Error.Printf("http server: %v: /etcdSet call cant be processes, no key supplied in %v", transID, r.URL)
			w.Write(output)
			return
		}

		server, err := c.GetRandomServer()
		if err != nil {
			output = generateErrorPayload(transID, http.StatusInternalServerError, err)
			w.Write(output)
			logger.Error.Printf("http server: %v: error %v was sent to the client: %v\n", transID, err, r.RemoteAddr)
			return
		}

		logger.Trace.Printf("http server: %v: random server selected for request processing: %v:%v\n", transID, server.ID, server.Name)
		data, err := server.Get(key, wait)
		logger.Trace.Printf("http server: %v: []byte received from the Etcd cluster: %v\n", transID, string(data[:]))
		if err != nil {
			output = generateErrorPayload(transID, http.StatusNotFound, err)
			logger.Error.Printf("http server: %v: error getting the value for the key %v: %v\n", transID, key, err)
		} else {

			var response common.EtcdSetCommandResponse
			err = common.Convert(data, &response)
			output, err = json.MarshalIndent(response, "", "    ")
			if err != nil {
				output = generateErrorPayload(transID, http.StatusInternalServerError, err)
				logger.Error.Printf("http server: %v: error marshalling Etcd server output/object into JSON: %v\n", transID, err)
			}
		}

		w.Write(output)
		logger.Info.Printf("http server: %v: response was sent to %v client: %v\n", transID, r.RemoteAddr, string(output[:]))
	}
}

// EtcdSetKeyValuePair a simple Key=Value pair in the Etcd cluster
func EtcdSetKeyValuePair(c *common.Cluster, logger myLog.LevelLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		transID := parseTransactionID(r.URL.Path)
		query := r.URL.Query()
		logger.Info.Printf("http server: %v: /etcdSet request (%v) received from %v\n", transID, query, r.RemoteAddr)
		var output []byte

		if len(query) == 0 {
			errorStr := fmt.Errorf("empty query passed in the URI %v, nothing to set", r.URL)
			output = generateErrorPayload(transID, http.StatusBadRequest, errorStr)
			w.Write(output)
			logger.Error.Printf("http server: %v: /etcdSet request failed: %v\n", transID, errorStr)
			return
		}

		key := query.Get("key")
		value := query.Get("value")
		ttlStr := query.Get("ttl")
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil {
			output = generateErrorPayload(transID, http.StatusBadRequest, err)
			w.Write(output)
			logger.Error.Printf("http server: %v: incorrect value for the TTL key (%v) provided: %v\n", transID, ttlStr, err)
			return
		}

		logger.Trace.Printf("http server: %v: key/value pair to be set in Etcd: %v:%v with ttl of %v\n", transID, key, value, ttl)
		if key != "" {
			server, err := c.GetRandomServer()
			if err != nil {
				output = generateErrorPayload(transID, http.StatusInternalServerError, err)
				w.Write(output)
				logger.Error.Printf("http server: %v: error %v was sent to the client %v\n", transID, err, r.RemoteAddr)
				return
			}
			logger.Trace.Printf("http server: %v: random server for request processing selected: %v:%v\n", transID, server.ID, server.Name)

			data, err := server.Set(key, value, ttl)
			logger.Trace.Printf("http server: %v: []byte response from Etcd received: %v\n", transID, string(data[:]))
			if err != nil {
				output = generateErrorPayload(transID, http.StatusInternalServerError, err)
				w.Write(output)
				logger.Error.Printf("http server: %v: error %v was sent to the client %v\n", transID, err, r.RemoteAddr)
				return
			}

			var setResponse common.EtcdSetCommandResponse
			err = common.Convert(data, &setResponse)
			output, err = json.MarshalIndent(setResponse, "", "    ")
			if err != nil {
				output = generateErrorPayload(transID, http.StatusInternalServerError, err)
				logger.Error.Printf("http server: %v: error marshalling server output/object into the JSON: %v\n", transID, err)
			}
		} else {
			errorStr := fmt.Errorf("empty key passed in the URI %v, cant set an empty item in the Etcd", r.URL)
			output = generateErrorPayload(transID, http.StatusBadRequest, errorStr)
			logger.Error.Printf("http server: %v: error processing the request: %v\n", transID, errorStr)
		}

		w.Write(output)
		logger.Info.Printf("http server: %v: response was sent to %v client: %v\n", transID, r.RemoteAddr, string(output[:]))
	}
}
