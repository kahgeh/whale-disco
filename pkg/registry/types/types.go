package types

import (
	"hash/fnv"
	"sort"
	"strings"
	"time"
)

type PluginType string

const (
	PluginDocker PluginType = "Docker"
)

// Endpoint represent the service endpoint
type Endpoint struct {
	UniqueID       string
	ClusterName    string
	Port           uint32
	Host           string
	FrontProxyPath string
	Version        string
}

// EndpointUpdateRequest represent the update request
type EndpointUpdateRequest struct {
	PluginName string
	Timestamp  time.Time
	Endpoints  []Endpoint
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// GetHash provide an indication if endpoints are the same from another set of endpoints
func (request *EndpointUpdateRequest) GetHash() uint32 {
	var ids []string
	for _, endpoint := range request.Endpoints {
		ids = append(ids, endpoint.UniqueID)
	}
	sort.Strings(ids)
	return hash(strings.Join(ids, ""))
}

func (request *EndpointUpdateRequest) GroupByCluster() map[string][]Endpoint {
	clusters := make(map[string][]Endpoint)
	for _, endpoint := range request.Endpoints {
		if _, alreadyExist := clusters[endpoint.ClusterName]; !alreadyExist {
			clusters[endpoint.ClusterName] = []Endpoint{endpoint}
			continue
		}
		endpoints := clusters[endpoint.ClusterName]
		clusters[endpoint.ClusterName] = append(endpoints, endpoint)
	}
	return clusters
}
