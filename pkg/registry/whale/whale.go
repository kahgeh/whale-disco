package whale

import (
	"fmt"

	"regexp"
	"strconv"
	"strings"
	"time"

	dTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	dClient "github.com/docker/docker/client"
	"github.com/kahgeh/whale-disco/pkg/ctx"

	"github.com/docker/docker/api/types/filters"
	"github.com/kahgeh/whale-disco/pkg/logger"
	"github.com/kahgeh/whale-disco/pkg/registry/types"
)

// Docker provides configuration source from whale
type Session struct {
	api *dClient.Client
}

type enPorts []dTypes.Port

type ContainerEvent string

const (
	EventStart ContainerEvent = "start"
	EventDie   ContainerEvent = "die"
)

const (
	commitIDKey = "COMMIT_ID"
	versionKey  = "VERSION"
)

var (
	portGroupExpr          = "(?P<port>\\d+)"
	urlPrefixExpr          = fmt.Sprintf("CLUSTER_%s_URLPREFIX", portGroupExpr)
	urlPrefixPattern       = regexp.MustCompile(urlPrefixExpr)
	serviceNameExpr        = fmt.Sprintf("CLUSTER_%s_NAME", portGroupExpr)
	serviceNamePattern     = regexp.MustCompile(serviceNameExpr)
	serviceCategoryExpr    = fmt.Sprintf("CLUSTER_%s_CATEGORY", portGroupExpr)
	serviceCategoryPattern = regexp.MustCompile(serviceCategoryExpr)
)

var (
	serviceNameSubmatchGroupLookup = toMap(serviceNamePattern.SubexpNames())
	portIndex                      = serviceNameSubmatchGroupLookup["port"]
)

type service struct {
	name      string
	category  string
	urlPrefix string
	version   string
	port      uint16
}

type discoverableContainer struct {
	services  []service
	container dTypes.Container
	ports     []dTypes.Port
}

func toMap(texts []string) map[string]int {
	m := make(map[string]int)

	for index, text := range texts {
		m[text] = index
	}
	return m
}

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}

func getServicePorts(container dTypes.Container) []uint16 {
	ports := []uint16{}
	uniquePortsContainer := make(map[uint16]string)
	for key := range container.Labels {
		if serviceNamePattern.MatchString(key) {
			submatches := serviceNamePattern.FindStringSubmatch(key)
			port := uint16(mustAtoi(submatches[portIndex]))
			if _, alreadyExists := uniquePortsContainer[port]; !alreadyExists {
				ports = append(ports, port)
				uniquePortsContainer[port] = "exist"
			}
		}
	}
	return ports
}

func mapContainerToDiscoverableContainer(container dTypes.Container, servicePorts []uint16) *discoverableContainer {
	labels := container.Labels
	discoveredContainer := &discoverableContainer{
		container: container,
	}
	var services []service
	for _, port := range servicePorts {
		serviceNameLabelKey := strings.Replace(serviceNameExpr, portGroupExpr, strconv.Itoa(int(port)), 1)
		serviceCategoryLabelKey := strings.Replace(serviceCategoryExpr, portGroupExpr, strconv.Itoa(int(port)), 1)
		urlPrefixLabelKey := strings.Replace(urlPrefixExpr, portGroupExpr, strconv.Itoa(int(port)), 1)

		service := service{
			name:      labels[serviceNameLabelKey],
			category:  labels[serviceCategoryLabelKey],
			urlPrefix: labels[urlPrefixLabelKey],
			version:   fmt.Sprintf("v%s-%s", labels[versionKey], labels[commitIDKey]),
			port:      port,
		}
		services = append(services, service)
	}
	discoveredContainer.services = services
	return discoveredContainer
}

func getDiscoverableContainers(containers []dTypes.Container) []discoverableContainer {
	var discoveredContainers []discoverableContainer
	for _, container := range containers {
		servicePorts := getServicePorts(container)
		if len(servicePorts) > 0 {
			discoveredContainers = append(discoveredContainers,
				*mapContainerToDiscoverableContainer(container, servicePorts))
		}
	}
	return discoveredContainers
}

func (ports enPorts) wherePorts(predicate func(dTypes.Port) bool) []dTypes.Port {
	var matchingPorts []dTypes.Port
	for _, port := range ports {
		if predicate(port) {
			matchingPorts = append(matchingPorts, port)
		}
	}
	return matchingPorts
}

func (ports enPorts) getMappedAddress(portNumber uint16) (mappedHost string, mappedPortNumber uint16) {
	mappedPorts := ports.
		wherePorts(func(p dTypes.Port) bool {
			return p.PrivatePort == portNumber
		})
	if len(mappedPorts) > 0 {
		mappedHost = "127.0.0.1"
		mappedPortNumber = mappedPorts[0].PrivatePort
	}
	return
}

func (container *discoverableContainer) mapToEndpoints() []types.Endpoint {
	var endpoints []types.Endpoint
	dockerContainer := container.container
	for _, service := range container.services {
		//todo : cleanup host
		host, portNumber := enPorts(dockerContainer.Ports).
			getMappedAddress(service.port)
		host = dockerContainer.NetworkSettings.Networks["bridge"].IPAddress
		frontProxyPath := fmt.Sprintf("/%s/%s", service.category,
			service.name)
		if service.urlPrefix != "" {
			frontProxyPath = fmt.Sprintf("/%s/%s", service.category,
				service.urlPrefix)
		}

		endpoint := types.Endpoint{
			UniqueID:       dockerContainer.ID,
			ClusterName:    service.name,
			Host:           host,
			Port:           uint32(portNumber),
			FrontProxyPath: frontProxyPath,
			Version:        service.version,
		}
		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

func (session *Session) waitForCompletion(evt events.Message) {
	//todo: properly wait
	<-time.After(5 * time.Second)
}

func (session *Session) getEndpointUpdateRequest() *types.EndpointUpdateRequest {
	appContext := ctx.GetContext()
	log := logger.New("getEndpointUpdateRequest")
	defer log.LogDone()
	api := session.api
	filters := filters.NewArgs()
	filters.Add("status", "running")
	containers, err := api.ContainerList(appContext, dTypes.ContainerListOptions{
		Filters: filters,
	})
	if err != nil {
		log.Warnf("error listing container, %s", err.Error())
		return nil
	}

	discoveredContainers := getDiscoverableContainers(containers)

	var endpoints []types.Endpoint
	for _, discoveredContainer := range discoveredContainers {
		endpoints = append(endpoints, discoveredContainer.mapToEndpoints()...)
	}

	updateRequest := &types.EndpointUpdateRequest{
		PluginName: string(types.PluginDocker),
		Timestamp:  time.Now(),
		Endpoints:  endpoints,
	}

	return updateRequest
}

func New() *Session {
	log := logger.New("connectToDocker")
	defer log.LogDone()
	dockerApi, err := dClient.NewEnvClient()
	if err != nil {
		panic(err)
	}
	return &Session{
		api: dockerApi,
	}
}

func (session *Session) Run() chan *types.EndpointUpdateRequest {
	log := logger.New("runDockerRegistry")
	defer log.LogDone()
	appContext := ctx.GetContext()
	api := session.api

	filters := filters.NewArgs()
	filters.Add("event", string(EventStart))
	filters.Add("event", string(EventDie))
	eventsOptions := dTypes.EventsOptions{
		Filters: filters,
	}
	log.Info("connecting to events channel...")
	eventsChannel, errChannel := api.Events(appContext, eventsOptions)
	log.Info("connected to events channel")
	updateRequestChannel := make(chan *types.EndpointUpdateRequest)
	go func(session *Session) {
		defer close(updateRequestChannel)
		errCnt := 0
		for {
			log.Info("get update request update request")
			updateRequest := session.getEndpointUpdateRequest()
			if updateRequest == nil {
				errCnt = errCnt + 1
				continue
			}
			select {
			case updateRequestChannel <- updateRequest:
				log.Info("sending update request")
			case <-appContext.Done():
				log.Info("terminating scanner loop")
				return
			}

			log.Debug("waiting for a whale event...")
			select {
			case evt := <-eventsChannel:
				log.Infof("received %q event from %v", evt.Action, evt.From)
				session.waitForCompletion(evt)
			case err := <-errChannel:
				log.Infof("error detected from event system, %s", err.Error())
				errCnt = errCnt + 1
			case <-appContext.Done():
				log.Infof("exiting after receiving request to end event loop")
				return
			}
			if errCnt > 10 {
				log.Fail("too many errors detected")
			}
		}
	}(session)
	return updateRequestChannel
}
