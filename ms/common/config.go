package common

// Config struct defines the layout of the config file (./config)
// it stores all the configuration attributes for the service and its components/modules
type Config struct {
	Name                   string            `json:"name"`
	TTL                    int               `json:"ttl"`
	Version                string            `json:"version"`
	HTTPServerHost         string            `json:"http_server_host,omitempty"`
	HTTPServerPort         string            `json:"http_server_port"`
	HTTPServerReadTimeout  int               `json:"http_server_read_timeout"`
	HTTPServerWriteTimeout int               `json:"http_server_write_timeout"`
	Logger                 map[string]string `json:"logger"`
	Service                Service           `json:"service"`
	Etcd                   etcdConfig        `json:"etcd"`
	Endpoints              MSEndpoints       `json:"endpoints"`
}

// MSEndpoints contains lists of endpoints for general, etcd and service sections for the microservice
type MSEndpoints struct {
	General []string `json:"general,omitempty"`
	Etcd    []string `json:"etcd,omitempty"`
	Service []string `json:"service,omitempty"`
}

// Service contains all the data that relates to the actual service
type Service struct {
	RedisHost     string `json:"redis_server_ip"`
	RedisPort     string `json:"redis_server_port"`
	OrderSetName  string `json:"redis_server_order_set"`
	OrderListName string `json:"redis_server_order_list"`
	WaitTime      int    `json:"order_check_wait_time"`
}

// etcdConfig is a sub-section containing values for the Etcd cluster configuration
type etcdConfig struct {
	Connection           etcdConnectionParams `json:"connection"`
	SleepBetweenAttempts int                  `json:"sleep_between_attempts"`
	NumberOfRetries      int                  `json:"number_retries"`
	Cluster              etcdCluster          `json:"cluster"`
}

type etcdConnectionParams struct {
	MaxIdleConns    int `json:"max_idle_conns"`
	IdleConnTiemout int `json:"idle_conn_timeout"`
	Timeout         int `json:"timeout"`
}

type etcdCluster struct {
	EtcdServers []etcdServer `json:"servers"`
}

type etcdServer struct {
	ClientEndpoints []string `json:"client_endpoints"`
	ServerEndpoints []string `json:"server_endpoints"`
}
