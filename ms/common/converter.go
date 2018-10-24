package common

import (
	"encoding/json"
	"fmt"
)

// Convert takes the []byte as input and converts it to the JSON representation and into the appropriate object as a result
// the input arguments are
//	1. data []byte - array with the actual object payload JSON
//	2. resultType interface{} - the object to be used to determine type and convert JSON into
// The function returns nil in case of success (the actual object is passed back via the &resultType) or error object with fmt.Errorf string
//
func Convert(data []byte, resultType interface{}) error {
	err := json.Unmarshal(data, &resultType)
	if err != nil {
		return fmt.Errorf("failed to unmarshal %q: %v", data, err)
	}
	return nil
}

// EtcdVersionPayload represents the actual struct that contains the version information
type EtcdVersionPayload struct {
	EtcdServer  string `json:"etcdserver"`
	EtcdCluster string `json:"etcdcluster"`
}

// EtcdSelfPayload contains the representation of the /v2/stats/self call payload to be used for common.server values
type EtcdSelfPayload struct {
	Name                 string         `json:"name"`
	ID                   string         `json:"id"`
	State                string         `json:"state"`
	StartTime            string         `json:"startTime"`
	LeaderInfo           etcdLeaderInfo `json:"leaderInfo"`
	RecvAppendRequestCnt int            `json:"recvAppendRequestCnt"`
	SendAppendRequestCnt int            `json:"sendAppendRequestCnt"`
	RecvBandwidthRate    float64        `json:"recvBandwidthRate,omitempty"`
	RecvPkgRate          float64        `json:"recvPkgRate,omitempty"`
}

type etcdLeaderInfo struct {
	ID        string `json:"leader"`
	Uptime    string `json:"uptime"`
	StartTime string `json:"startTime"`
}

// EtcdMembersList contains the structure representing the Etcd Cluster member list
type EtcdMembersList struct {
	Members []EtcdClusterMember `json:"members"`
}

// EtcdClusterMember represents individual member in the Etcd Cluster
type EtcdClusterMember struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	PeerURLs   []string `json:"peerURLs"`
	ClientURLs []string `json:"clientURLs"`
}

// EtcdGetCommandResponse contains a struct that allows to unmarshal the repsonse from Etcd server for executing Set(key:value) and Get pair
type EtcdGetCommandResponse struct {
	Action   string   `json:"action"`
	Node     EtcdNode `json:"node"`
	PrevNode EtcdNode `json:"prevNode,omitempty"`
}

// EtcdNode represents a Node object in Etcd
type EtcdNode struct {
	Key           string `json:"key"`
	Value         string `json:"value,omitempty"`
	Dir           bool   `json:"dir,omitempty"`
	ModifiedIndex int    `json:"modifiedIndex"`
	CreatedIndex  int    `json:"createdIndex"`
	Expiration    string `json:"expiration,omitempty"`
	TTL           int    `json:"ttl,omitempty"`
}

// ResponsePayload indicates that an error occurred and what are the error details
type ResponsePayload struct {
	TransactionID string `json:"transactionId,omitempty"`
	Status        int    `json:"status"`
	Message       string `json:"message"`
}
