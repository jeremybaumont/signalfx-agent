// Package docker is an observer that watches a docker daemon and reports
// container ports as service endpoints.
package docker

import (
	"context"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	dockercommon "github.com/signalfx/signalfx-agent/internal/core/common/docker"

	"github.com/signalfx/signalfx-agent/internal/core/config"
	"github.com/signalfx/signalfx-agent/internal/core/services"
	"github.com/signalfx/signalfx-agent/internal/observers"
)

const (
	observerType     = "docker"
	dockerAPIVersion = "v1.22"
)

// OBSERVER(docker): Queries the Docker Engine API for running containers.  If
// you are using Kubernetes, you should use the [k8s-api
// observer](./k8s-api.md) instead of this.
//
// Note that you will need permissions to access the Docker engine API.  For a
// Docker domain socket URL, this means that the agent needs to have read
// permissions on the socket.  We don't currently support authentication for
// HTTP URLs.
//
// ## Configuration from Labels
// You can configure monitors by putting special labels on your Docker
// containers.  You can either specify all of the configuration in container
// labels, or you can use the more traditional agent configuration with
// discovery rules and specify configuration overrides with labels.
//
// The config labels are of the form `agent.signalfx.com.config.<port
// number>.<config_key>: <config value>`.  The `<config value>` must be a
// string in a container label, but it will be deserialized as a YAML value to
// the most appropriate type when consumed by the agent.  For example, if you
// have a Redis container and want to monitor it at a higher frequency than
// other Redis containers, you could have an agent config that looks like the
// following:
//
// ```
// observers:
//  - type: docker
// monitors:
//  - type: collectd/redis
//    discoveryRule: container_image =~ "redis" && port == 6379
//    auth: mypassword
//    intervalSeconds: 10
// ```
//
// And then launch the Redis container with the label:
//
// `agent.signalfx.com.config.6379.intervalSeconds`: `1`
//
// This would cause the config value for `intervalSeconds` to be overwritten to
// the more frequent 1 second interval.
//
// You can also specify the monitor configuration entirely with Docker labels
// and completely omit monitor config from the agent config.  With the agent
// config:
//
// ```
// observers:
//  - type: docker
// ```
//
// You can then launch a Redis container with the following labels:
//
//  - `agent.signalfx.com.monitorType.6379`: `collectd/redis`
//  - `agent.signalfx.com.config.6379.auth`: `mypassword`
//
// Which would configure a Redis monitor with the given authentication
// configuration.  No Redis configuration is required in the agent config file.
//
// The distinction is that the `monitorType` label was added to the Docker
// container.  If a `monitorType` label is present, **no discovery rules will
// be considered for this endpoint**, and thus, no agent configuration can be
// used anyway.
//
// ### Multiple Monitors per Port
// If you want to configure multiple monitors per port, you can specify the
// port name in the form `<port number>-<port name>` instead of just the port
// number.  For example, if you had two different Prometheus exporters running
// on the same port, but on different paths in a given container, you could
// provide labels like the following:
//
// ```
//  - `agent.signalfx.com.monitorType.8080-app`: `prometheus-exporter`
//  - `agent.signalfx.com.config.8080-app.metricPath`: `/appMetrics`
//  - `agent.signalfx.com.monitorType.8080-goruntime`: `prometheus-exporter`
//  - `agent.signalfx.com.config.8080-goruntime.metricPath`: `/goMetrics`
// ```
//
// The name that is given to the port will populate the `name` field of the
// discovered endpoint and can be used in discovery rules as such.  For
// example, with the following agent config:
//
// ```
// observers:
//  - type: docker
// monitors:
//  - type: prometheus-exporter
//    discoveryRule: name == "app" && port == 8080
//    intervalSeconds: 1
// ```
//
// And given docker labels as follows (remember that discovery rules are
// irrelevant to endpoints that specify `monitorType` labels):
//
//  - `agent.signalfx.com.config.8080-app.metricPath`: `/appMetrics`
//  - `agent.signalfx.com.config.8080-goruntime.metricPath`: `/goMetrics`
//
// Would result in the `app` endpoint getting an interval of 1 second and the
// `goruntime` endpoint getting the default interval of the agent.

// ENDPOINT_TYPE(ContainerEndpoint): true

var logger = log.WithFields(log.Fields{"observerType": observerType})

// Docker observer plugin
type Docker struct {
	serviceCallbacks *observers.ServiceCallbacks
	config           *Config
	cancel           func()

	endpointsByContainerID map[string][]services.Endpoint
}

// Config specific to the Docker observer
type Config struct {
	config.ObserverConfig
	DockerURL string `yaml:"dockerURL" default:"unix:///var/run/docker.sock"`
	// A mapping of container label names to dimension names that will get
	// applied to the metrics of all discovered services. The corresponding
	// label values will become the dimension values for the mapped name.  E.g.
	// `io.kubernetes.container.name: container_spec_name` would result in a
	// dimension called `container_spec_name` that has the value of the
	// `io.kubernetes.container.name` container label.
	LabelsToDimensions map[string]string `yaml:"labelsToDimensions"`
	// If true, the "Config.Hostname" field (if present) of the docker
	// container will be used as the discovered host that is used to configure
	// monitors.  If false or if no hostname is configured, the field
	// `NetworkSettings.IPAddress` is used instead.
	UseHostnameIfPresent bool `yaml:"useHostnameIfPresent"`
	// If true, the observer will configure monitors for matching container endpoints
	// using the host bound ip and port.  This is useful if containers exist that are not
	// accessible to an instance of the agent running outside of the docker network stack.
	UseHostBindings bool `yaml:"useHostBindings" default:"false"`
	// If true, the observer will ignore discovered container endpoints that are not bound
	// to host ports.  This is useful if containers exist that are not accessible
	// to an instance of the agent running outside of the docker network stack.
	IgnoreNonHostBindings bool `yaml:"ignoreNonHostBindings" default:"false"`
}

func init() {
	observers.Register(observerType, func(cbs *observers.ServiceCallbacks) interface{} {
		return &Docker{
			serviceCallbacks:       cbs,
			endpointsByContainerID: make(map[string][]services.Endpoint),
		}
	}, &Config{})
}

// Configure the docker client
func (docker *Docker) Configure(config *Config) error {
	defaultHeaders := map[string]string{"User-Agent": "signalfx-agent"}

	client, err := client.NewClient(config.DockerURL, dockerAPIVersion, nil, defaultHeaders)
	if err != nil {
		return errors.Wrapf(err, "Could not create docker client")
	}

	docker.config = config

	var ctx context.Context
	ctx, docker.cancel = context.WithCancel(context.Background())

	err = dockercommon.ListAndWatchContainers(ctx, client, docker.changeHandler, nil, logger)
	if err != nil {
		logger.WithError(err).Error("Could not list docker containers")
		return err
	}
	return nil
}

func (docker *Docker) changeHandler(old *dtypes.ContainerJSON, new *dtypes.ContainerJSON) {
	var newEndpoints []services.Endpoint
	var oldEndpoints []services.Endpoint

	if old != nil {
		oldEndpoints = docker.endpointsByContainerID[old.ID]
		delete(docker.endpointsByContainerID, old.ID)
	}

	if new != nil {
		newEndpoints = docker.endpointsForContainer(new)
		docker.endpointsByContainerID[new.ID] = newEndpoints
	}

	// Prevent spurious churn of endpoints if they haven't changed
	if reflect.DeepEqual(newEndpoints, oldEndpoints) {
		return
	}

	// If it is an update, there will be a remove and immediately subsequent
	// add.
	for i := range oldEndpoints {
		log.Debugf("Removing Docker endpoint from container %s", old.ID)
		docker.serviceCallbacks.Removed(oldEndpoints[i])
	}

	for i := range newEndpoints {
		log.Debugf("Adding Docker endpoint for container %s", new.ID)
		docker.serviceCallbacks.Added(newEndpoints[i])
	}
}

// Discover services by querying docker api
func (docker *Docker) endpointsForContainer(cont *dtypes.ContainerJSON) []services.Endpoint {
	instances := make([]services.Endpoint, 0)

	if cont.State.Running && !cont.State.Paused {
		serviceContainer := &services.Container{
			ID:      cont.ID,
			Names:   []string{cont.Name},
			Image:   cont.Config.Image,
			Command: strings.Join(cont.Config.Cmd, " "),
			State:   cont.State.Status,
			Labels:  cont.Config.Labels,
		}

		labelConfigs := getConfigLabels(cont.Config.Labels)
		knownPorts := map[contPort]bool{}

		for port := range labelConfigs {
			knownPorts[port] = true
		}

		for k := range cont.Config.ExposedPorts {
			knownPorts[contPort{Port: k}] = true
		}

		for portObj := range knownPorts {

			endpoint := docker.endpointForPort(portObj, cont, serviceContainer)

			// the endpoint was not set, so we'll drop it
			if endpoint == nil {
				continue
			}

			if labelConf := labelConfigs[portObj]; labelConf != nil {
				endpoint.MonitorType = labelConf.MonitorType
				endpoint.Configuration = labelConf.Configuration
			}

			instances = append(instances, endpoint)
		}
	}

	return instances
}

func (docker *Docker) endpointForPort(portObj contPort, cont *dtypes.ContainerJSON, serviceContainer *services.Container) *services.ContainerEndpoint {
	port := portObj.Int()
	protocol := portObj.Proto()

	mappedPort, mappedIP := dockercommon.FindHostMappedPort(cont, portObj.Port)

	// if IgnoreNonHostBindings is set to true and there isn't a host binding
	// return nil to skip this endpoint
	if docker.config.IgnoreNonHostBindings && mappedPort == 0 && mappedIP == "" {
		return nil
	}

	id := serviceContainer.PrimaryName() + "-" + cont.ID[:12] + "-" + strconv.Itoa(int(port))
	if portObj.Name != "" {
		id += "-" + portObj.Name
	}

	orchDims := map[string]string{}
	for k, dimName := range docker.config.LabelsToDimensions {
		if v := cont.Config.Labels[k]; v != "" {
			orchDims[dimName] = v
		}
	}

	endpoint := &services.ContainerEndpoint{
		EndpointCore:  *services.NewEndpointCore(id, portObj.Name, observerType, orchDims),
		Container:     *serviceContainer,
		Orchestration: *services.NewOrchestration("docker", services.DOCKER, services.PRIVATE),
	}

	if docker.config.UseHostnameIfPresent && cont.Config.Hostname != "" {
		endpoint.Host = cont.Config.Hostname
	} else {
		// Use the IP Address of the first network we iterate over.
		// This can be made configurable if so desired.
		for _, n := range cont.NetworkSettings.Networks {
			endpoint.Host = n.IPAddress
			break
		}
	}

	endpoint.PortType = services.PortType(strings.ToUpper(protocol))

	if docker.config.UseHostBindings && mappedPort != 0 && mappedIP != "" {
		endpoint.Orchestration.PortPref = services.PUBLIC
		endpoint.Port = uint16(mappedPort)
		endpoint.AltPort = uint16(port)
		endpoint.Host = mappedIP
		if endpoint.Host == "0.0.0.0" {
			endpoint.Host = "127.0.0.1"
		}
	} else {
		endpoint.Port = uint16(port)
		endpoint.AltPort = uint16(mappedPort)
	}

	return endpoint
}

// Shutdown the service differ routine
func (docker *Docker) Shutdown() {
	if docker.cancel != nil {
		docker.cancel()
	}
}
