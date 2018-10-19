package common

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// server is a class for representing the server details
/*
	ClientEndpoint is an URI to client in format of http://x.x.x.x:yyyy
	the yyyy is the client port, by default etcd is using 2379
	ServerEndpoint is the server-to-server communication URI in format of http://x.x.x.x:zzzz
	for the port portion, by default etcd is using 2380
*/
type server struct {
	ID                   string
	Name                 string
	StateLeader          bool
	ClientEndpoints      []string
	ServerEndpoints      []string
	RecvAppendRequestCnt int
	RecvBandwidthRate    float64
	RecvPkgRate          float64
	SendAppendRequestCnt int
	client               *http.Client
}

// Connect creates a client object needed to run the HTTP calls with and also initiates the /v2/stats/self call
// to collect additional details about the server object, such as Etcd Id and Name of the member, even in case of
// non-cluster standalone setup
// The Connect timeout to Etcd is pre-set to 30 secs
func (s *server) Connect(config Config) error {
	transport := &http.Transport{
		MaxIdleConns:    config.Etcd.Connection.MaxIdleConns,
		IdleConnTimeout: time.Duration(config.Etcd.Connection.IdleConnTiemout) * time.Second,
	}
	s.client = &http.Client{
		Transport: transport,
		Timeout:   time.Duration(config.Etcd.Connection.Timeout) * time.Second,
	}
	return s.Self()
}

// Self function runs the /v2/self call to get the ID and Name of each individual server
func (s *server) Self() error {
	failedAttempts := 0

	for _, endpoint := range s.ClientEndpoints {
		url := endpoint + "/v2/stats/self"
		request, _ := http.NewRequest(http.MethodGet, url, nil)
		response, doErr := s.client.Do(request)
		if doErr == nil && response != nil {
			// this endpoint good, we will produce some additional details to the server object
			defer response.Body.Close()

			data, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return fmt.Errorf("error processing the output from the %v call: %v", url, err)
			}

			var selfObj EtcdSelfPayload
			converErr := Convert(data, &selfObj)
			if converErr != nil {
				return fmt.Errorf("error processing the JSON payload from the %v call: %v", url, err)
			}
			s.ID = selfObj.ID
			s.Name = selfObj.Name
			s.RecvBandwidthRate = selfObj.RecvBandwidthRate
			s.RecvAppendRequestCnt = selfObj.RecvAppendRequestCnt
			s.RecvPkgRate = selfObj.RecvPkgRate
			s.SendAppendRequestCnt = selfObj.SendAppendRequestCnt
			if selfObj.State == "StateLeader" {
				s.StateLeader = true
			}
			break
		}
		failedAttempts++
	}

	if failedAttempts == len(s.ClientEndpoints) {
		errorReturn := fmt.Errorf("error connecting to the client endpoints %v for the server, connection failed", s.ClientEndpoints)
		s.ClientEndpoints = []string{}
		return errorReturn
	}

	return nil
}

// Set is a generic method for setting a key-value pair
// returns a []byte representing the response from the server in case of success, error object otherwise
// ttl is used to set Time-to-Live value in Etcd
func (s *server) Set(key, value string, ttl int) ([]byte, error) {
	failedAttempts := 0
	errStr := make([]string, 0, 5)
	var data []byte
	var readErr error

	for _, endpoint := range s.ClientEndpoints {
		request, err := http.NewRequest(http.MethodPut, endpoint+"/v2/keys"+key, strings.NewReader("value="+value+"&ttl="+strconv.Itoa(ttl)))
		request.Header.Set("content-type", "application/x-www-form-urlencoded")
		if err != nil {
			return nil, fmt.Errorf("error occured when creating new PUT request to %v endpoint to create a %v 'key' and %v 'value' with error: %v",
				endpoint, key, value, err)
		}

		response, responseErr := s.client.Do(request)
		if response.StatusCode == http.StatusNotFound {
			errStr = append(errStr, fmt.Errorf("the /etcdSet %v:%v failed with status: %v", key, value, response.Status).Error())
			return nil, fmt.Errorf("error adding a %v=%v in the Etcd server %v: %v", key, value, s.ID, strings.Join(errStr, ", "))
		}

		if responseErr != nil {
			errStr = append(errStr, responseErr.Error())
			failedAttempts++
		} else {
			defer response.Body.Close()
			data, readErr = ioutil.ReadAll(response.Body)
			if readErr != nil {
				return nil, fmt.Errorf("error when parsing the response from %v endpoint from Set(%v=%v) command: %v",
					endpoint, key, value, readErr)
			}
			break
		}

	}

	if failedAttempts == len(s.ClientEndpoints) {
		return nil, fmt.Errorf("error adding a %v=%v in the Etcd server %v: %v", key, value, s.ID, strings.Join(errStr, ", "))
	}

	return data, nil
}

// Get call uses the key parameter to query Etcd cluster for a key and returns a value that's being found
// the wait attribute is specific to the Etcd cluster and allows to run a request in the "wait" mode / long polling
// to "listen" for the changes for a specific key.
func (s *server) Get(key string, wait bool) ([]byte, error) {
	failedAttempt := 0

	for _, endpoint := range s.ClientEndpoints {
		request, err := http.NewRequest(http.MethodGet, endpoint+"/v2/keys"+key+"?wait="+strconv.FormatBool(wait), nil)
		if err != nil {
			failedAttempt++
		}

		ch := make(chan *http.Response)
		go func(wait bool, ch chan *http.Response) {
			response, err := s.client.Do(request)
			if err != nil {
				ch <- nil
			}
			ch <- response
		}(wait, ch)

		response := <-ch
		if response != nil {
			defer response.Body.Close()
			data, err := ioutil.ReadAll(response.Body)
			data = bytes.Trim(data, "\n")

			if response.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("the key %v is not found in the Etcd, server response is status code: %v", key, string(data[:]))
			}

			if err != nil {
				return nil, fmt.Errorf("processing of the %v/v2/keys%v response failed with: %v", endpoint, key, err)
			}
			return data, nil
		}

		if failedAttempt == len(s.ClientEndpoints) {
			break
		}
	}

	return nil, fmt.Errorf("error trying to construct a request /v2/keys/... to retrieve a value for a %v key", key)
}

/*
Version returns the current etcd version or error if the GET call fails
in case of error, the first attribute is nil
The return values could be:
	1. (nil, error) in cases when various errors happen, the error is formatted with fmt.Errorf to pass additioanl details
	2. ([]byte, nil) in case of success - the slide of bytes is passed back and error is nil
*/
func (s *server) Version() ([]byte, error) {
	failedAttempts := 0

	for _, endpoint := range s.ClientEndpoints {
		request, err := http.NewRequest(http.MethodGet, endpoint+"/version", nil)
		if err != nil {
			failedAttempts++
		}

		response, err := s.client.Do(request)
		if err != nil {
			failedAttempts++
		}

		if response != nil && response.StatusCode == http.StatusOK {
			defer response.Body.Close()
			data, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return nil, fmt.Errorf("processing of the %v/version response failed with: %v", endpoint, err)
			}
			return data, nil
		}

		if failedAttempts == len(s.ClientEndpoints) {
			break
		}
	}

	return nil, fmt.Errorf("request for the Etcd version failed, the Etcd cluster is not running or not reachable")
}

// NewServer creates a new server object and sets the values required to establish connection
func NewServer(clientURI, serverURI []string) server {
	server := server{}
	server.ClientEndpoints = clientURI
	server.ServerEndpoints = serverURI
	return server
}

// Cluster is a struct that represents the whole cluster, with individual Server nodes
type Cluster struct {
	servers map[string]server
}

// NewCluster function creates a new Cluster instance and initializes the server map
func NewCluster() Cluster {
	g := Cluster{
		servers: make(map[string]server),
	}
	return g
}

// AddEndpoint adds a new server instance to the Cluster object
func (c *Cluster) AddEndpoint(endpoint server) {
	if endpoint.ID != "" {
		c.servers[endpoint.ID] = endpoint
	}
}

// RegisterEndpoints adds multiple endpoints at a time
func (c *Cluster) RegisterEndpoints(endpoints []interface{}) {
	for _, endpoint := range endpoints {
		c.AddEndpoint(endpoint.(server))
	}
}

// GetServersList returns a map of servers in the cluster
func (c *Cluster) GetServersList() *map[string]server {
	return &c.servers
}

// GetNumberOfServers returns how many active servers are there in the cluster
func (c *Cluster) GetNumberOfServers() int {
	return len(c.servers)
}

// GetRandomServer allows to implement a lightweight "round-robin" mechanism with selecting random server from the map
// this should distribute the load across all the available Etcd nodes
// If no servers are active in the cluster - the error is passed back
func (c *Cluster) GetRandomServer() (*server, error) {
	if c.GetNumberOfServers() == 0 {
		return nil, fmt.Errorf("no active Etcd server in the cluster or servers are unreachable")
	}

	ids := make([]*server, 0, 3)

	randomizer := rand.New(rand.NewSource(time.Now().UnixNano()))
	randIndex := randomizer.Intn(len(c.servers))
	for k := range c.servers {
		serv := c.servers[k]
		ids = append(ids, &serv)
	}
	return ids[randIndex], nil
}

// UpdateMembersRoutine run the go routing to constantly refresh the list of servers for a cluster
// It allows to pass the number of fail attempts and sleep time between attempts. If the negative value is passed
// for the number of attempts, it will continue indefinitely
func (c *Cluster) UpdateMembersRoutine(config Config, ch chan error) {
	iters := 0

	if config.Etcd.NumberOfRetries > -1 {
		iters = config.Etcd.NumberOfRetries
	}
	go func(index int) {
		tried := 0
		for {
			err := c.UpdateMembers(config)
			if err != nil {
				// we hit the error, for now we increase the error counter
				tried++
			}
			if tried == index {
				// the error counter reached the max cap, we put message in the channel and break the loop
				ch <- err
				break
			}
			time.Sleep(time.Duration(config.Etcd.SleepBetweenAttempts) * time.Second)
		}
	}(iters)
}

// UpdateMembers is a function that allows to refresh the list of endpoints in the cluster based on the  member call
// It loops through the list of server object that it maintains and refreshes that list periodically by trying the
// /v2/members call.
// The method will fils with error if no endpoints on the list could be used to retrieve the updated list of members - that means
// no synch is possible and no endpoints/servers are responsive, some config changes might have happened prevented the state sync
func (c *Cluster) UpdateMembers(config Config) error {
	var updated bool
	errorList := make([]string, 0, 5)
	for _, server := range c.servers {
		for _, endpoint := range server.ClientEndpoints {
			url := endpoint + "/v2/members"
			request, _ := http.NewRequest(http.MethodGet, url, nil)
			response, err := server.client.Do(request)

			if err == nil && response != nil {
				// the call successed, the endpoint is valid still
				// we will use the output values to update the array of servers in the cluster
				defer response.Body.Close()

				responseData, readErr := ioutil.ReadAll(response.Body)
				if readErr != nil {
					return fmt.Errorf("error reading the response body of %v HTTP call occurred: %v", request.URL, readErr)
				}

				membersObj := EtcdMembersList{}
				convertErr := Convert(responseData, &membersObj)
				if convertErr != nil {
					return fmt.Errorf("error occurred converting the output JSON from the %v call: %v", url, err)
				}
				c.updateClusterMembers(config, &membersObj)
				updated = true
				break
			} else {
				errorList = append(errorList, err.Error())
			}
			if updated {
				break
			}
		}
	}

	if len(c.servers) == 0 || !updated {
		errorStr := strings.Join(errorList, ", ")
		return fmt.Errorf("error updating Etcd server list, cluster is not reachable or down: %v", errorStr)
	}

	return nil
}

// updateClusterMembers run the comparison of the list of servers in the cluster and compares them to what's passed by the
// actual call to list the registered members from the Etcd
// The logic is not ideal but allows to check the server map and compare it fully with the what's discovered
// the discovered servers will be added as new, the servers no longer discoverable are to be removed from the map
// TODO: rework with Mux to not change the map while someone is readting to avoid concurrent map iteration and map write
func (c *Cluster) updateClusterMembers(config Config, membersEtcd *EtcdMembersList) {
	var mutex = &sync.RWMutex{}
	idList := make(map[string]bool)

	for _, member := range membersEtcd.Members {
		serverObj := server{
			ServerEndpoints: member.PeerURLs,
			ClientEndpoints: member.ClientURLs,
		}
		serverObj.Connect(config)
		mutex.RLock()
		c.servers[serverObj.ID] = serverObj
		mutex.RUnlock()
		idList[serverObj.ID] = true
	}

	var currentKeys []string
	for k := range c.servers {
		currentKeys = append(currentKeys, k)
	}

	mutex.RLock()
	for _, key := range currentKeys {
		if _, ok := idList[key]; ok == false {
			delete(c.servers, key)
		}
	}
	delete(c.servers, "")
	mutex.RUnlock()
}
