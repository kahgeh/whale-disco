package mappers

import (
	"errors"
	"fmt"
	"github.com/kahgeh/whale-disco/pkg/logger"
	"time"

	"github.com/golang/protobuf/ptypes"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	rTypes "github.com/kahgeh/whale-disco/pkg/registry/types"
)

const ListenerPort = 10000

func mapToCluster(clusterName string) *cluster.Cluster {
	return &cluster.Cluster{
		Name:                      clusterName,
		ConnectTimeout:            ptypes.DurationProto(5 * time.Second),
		ClusterDiscoveryType:      &cluster.Cluster_Type{Type: cluster.Cluster_EDS},
		LbPolicy:                  cluster.Cluster_ROUND_ROBIN,
		IgnoreHealthOnHostRemoval: true,
		EdsClusterConfig: &cluster.Cluster_EdsClusterConfig{
			ServiceName: clusterName,
			EdsConfig:   makeConfigSource(),
		},
	}

}

func mapToClusters(clusterEndPoints map[string][]rTypes.Endpoint) []types.Resource {
	var clusters []types.Resource
	for name := range clusterEndPoints {
		clusters = append(clusters, mapToCluster(name))
	}
	return clusters
}

func mapToEndpointsResources(clusterEndPoints map[string][]rTypes.Endpoint) []types.Resource {
	var endpointResources []types.Resource
	for name, endpoints := range clusterEndPoints {
		endpointResources = append(endpointResources, mapToEndpoints(name, endpoints))
	}
	return endpointResources
}

func mapToEndpoints(clusterName string, clusterEndpoints []rTypes.Endpoint) *endpoint.ClusterLoadAssignment {
	log := logger.New("mapToEndpoints")
	defer log.LogDone()
	if len(clusterEndpoints) < 1 {
		return &endpoint.ClusterLoadAssignment{
			ClusterName: clusterName,
		}
	}
	var lbEndpoints []*endpoint.LbEndpoint
	for _, clusterEndpoint := range clusterEndpoints {
		lbEndpoints = append(lbEndpoints, mapToEndpoint(clusterEndpoint))
	}
	log.Infof("cluster %q has %v endpoints", clusterName, len(lbEndpoints))
	return &endpoint.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpoint.LocalityLbEndpoints{{
			LbEndpoints: lbEndpoints,
		}},
	}
}

func mapToEndpoint(clusterEndpoint rTypes.Endpoint) *endpoint.LbEndpoint {
	host := clusterEndpoint.Host
	port := clusterEndpoint.Port
	return &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				HealthCheckConfig: &endpoint.Endpoint_HealthCheckConfig{
					Hostname:  host,
					PortValue: port,
				},
				Address: &core.Address{
					Address: &core.Address_SocketAddress{
						SocketAddress: &core.SocketAddress{
							Protocol: core.SocketAddress_TCP,
							Address:  host,
							PortSpecifier: &core.SocketAddress_PortValue{
								PortValue: port,
							},
						},
					},
				},
			},
		},
	}
}

func mapToRoutes(clusterEndPoints map[string][]rTypes.Endpoint, routeName string, domainName string) []types.Resource {
	log := logger.New("mapToRoutes")
	defer log.LogDone()
	var routes []*route.Route
	for clusterName, endpoints := range clusterEndPoints {
		if endpoints == nil || len(endpoints) < 1 {
			continue
		}
		anyEndpoint := endpoints[0]
		prefix := anyEndpoint.FrontProxyPath
		log.Infof("%q's cluster is %s, with %v endpoints", prefix, clusterName, len(endpoints))
		routes = append(routes, mapToClusterRoutes(prefix, clusterName)...)
	}

	return []types.Resource{
		&route.RouteConfiguration{
			Name: routeName,
			VirtualHosts: []*route.VirtualHost{{
				Name:    "backend",
				Domains: []string{domainName},
				Routes:  routes,
			}},
		}}
}

func mapToClusterRoutes(prefix string, clusterName string) []*route.Route {

	return []*route.Route{{
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: fmt.Sprintf("%s/", prefix),
			},
		},
		Action: &route.Route_Route{
			Route: &route.RouteAction{
				ClusterSpecifier: &route.RouteAction_Cluster{
					Cluster: clusterName,
				},
			},
		}}, {
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: prefix,
			},
		},
		Action: &route.Route_Route{
			Route: &route.RouteAction{
				ClusterSpecifier: &route.RouteAction_Cluster{
					Cluster: clusterName,
				},
			},
		}},
	}
}

func makeConfigSource() *core.ConfigSource {
	source := &core.ConfigSource{}
	source.ResourceApiVersion = resource.DefaultAPIVersion
	source.ConfigSourceSpecifier = &core.ConfigSource_ApiConfigSource{
		ApiConfigSource: &core.ApiConfigSource{
			TransportApiVersion:       resource.DefaultAPIVersion,
			ApiType:                   core.ApiConfigSource_GRPC,
			SetNodeOnFirstMessageOnly: true,
			GrpcServices: []*core.GrpcService{{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &core.GrpcService_EnvoyGrpc{ClusterName: "xds_cluster"},
				},
			}},
		},
	}
	return source
}

func superset(names map[string]bool, resources map[string]types.Resource) error {
	for resourceName := range resources {
		if _, exists := names[resourceName]; !exists {
			return fmt.Errorf("%q not listed", resourceName)
		}
	}
	return nil
}

func Consistent(s *cache.Snapshot) error {
	if s == nil {
		return errors.New("nil snapshot")
	}
	endpoints := cache.GetResourceReferences(s.Resources[types.Cluster].Items)
	if len(endpoints) != len(s.Resources[types.Endpoint].Items) {
		return fmt.Errorf("mismatched endpoint reference and resource lengths: %v != %d", endpoints, len(s.Resources[types.Endpoint].Items))
	}
	if err := superset(endpoints, s.Resources[types.Endpoint].Items); err != nil {
		return err
	}
	return nil
}

func MapToSnapshot(clusterEndPoints map[string][]rTypes.Endpoint, version string, domainName string) (newSnapshot cache.Snapshot, err error) {
	routeName := "discovered_container_services"
	newSnapshot = cache.NewSnapshot(
		version,
		mapToEndpointsResources(clusterEndPoints), // endpoints
		mapToClusters(clusterEndPoints),
		mapToRoutes(clusterEndPoints, routeName, domainName),
		[]types.Resource{},
		[]types.Resource{}, // runtimes
	)

	return newSnapshot, Consistent(&newSnapshot)
}
