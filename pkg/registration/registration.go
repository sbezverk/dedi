package registration

import (
	"fmt"
	"net"
	"path"
	"strconv"
	"time"

	"github.com/sbezverk/dedi/pkg/tools"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	serverBasePath = pluginapi.DevicePluginPath
)

type resourceManager struct {
	socket   string
	listener net.Listener
	server   *grpc.Server
	stopCh   chan struct{}
	updateCh chan struct{}
	logger   *zap.SugaredLogger
}

// ResourceManager is interface to access resourceController resources
type ResourceManager interface {
	Run() error
	Shutdown()
}

// NewResourceManager creates an instance of a new resourceCOntroller and returns its interface
func NewResourceManager(logger *zap.SugaredLogger, updateCh chan struct{}) (ResourceManager, error) {
	var err error
	rm := resourceManager{
		logger:   logger,
		stopCh:   make(chan struct{}),
		updateCh: updateCh,
	}
	rm.socket = path.Join(serverBasePath, "dispatch-resource-controller.sock")
	// Preparing to start Resource Controller Device Plugin gRPC server
	if err = tools.SocketCleanup(rm.socket); err != nil {
		return nil, fmt.Errorf("Failed to cleaup stale socket with error: %+v", err)
	}
	// Setting up gRPC server
	rm.listener, err = net.Listen("unix", rm.socket)
	if err != nil {
		return nil, fmt.Errorf("Failed to setup listener with error: %+v", err)
	}
	rm.server = grpc.NewServer([]grpc.ServerOption{}...)
	// Attaching Device Plugin API
	pluginapi.RegisterDevicePluginServer(rm.server, &rm)

	return &rm, nil
}

func (rm *resourceManager) Run() error {
	// Starting Resource Controller gRPC server Device Plugin Server
	rm.logger.Infof("Starting Resource Controller gRPC server on socket: %s", rm.socket)

	go func() {
		if err := rm.server.Serve(rm.listener); err != nil {
			rm.logger.Errorf("Failed to start Resource Controller gRPC server on socket: %s with error: %+v", rm.socket, err)
		}
	}()
	// Wait for server to start by launching a blocking connexion
	rm.logger.Infof("Wait for Resource Controller gRPC server to become ready on socket: %s", rm.socket)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := tools.Dial(ctx, rm.socket)
	if err != nil {
		return err
	}
	defer conn.Close()
	rm.logger.Infof("Resource Controller gRPC server is ready and operational.")
	// Register Device Plugin with Kubernetes' local kubelet
	return register("dedi.io/dispatcher", rm.socket, rm.logger)
}

func (rm *resourceManager) Shutdown() {
	// Sending message to StopCh so Resource Controller withdraws resource advertisements
	rm.stopCh <- struct{}{}
	// Stopping gRPC server
	rm.server.Stop()
}

func (rm *resourceManager) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (rm *resourceManager) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (rm *resourceManager) buildDeviceList(health string) []*pluginapi.Device {
	deviceList := []*pluginapi.Device{}
	// Always advertise to kubelet 100 instances of descriptor dispatcher
	for i := 0; i < 100; i++ {
		device := pluginapi.Device{}
		device.ID = "dispatcher-" + strconv.Itoa(i)
		device.Health = health
		deviceList = append(deviceList, &device)
	}
	return deviceList
}

// ListAndWatch advertises to kubelet dispatcher as well as learned services
func (rm *resourceManager) ListAndWatch(e *pluginapi.Empty, d pluginapi.DevicePlugin_ListAndWatchServer) error {
	rm.logger.Info("Resource Controller's List and Watch was called")
	d.Send(&pluginapi.ListAndWatchResponse{Devices: rm.buildDeviceList(pluginapi.Healthy)})
	for {
		select {
		case <-rm.stopCh:
			// Informing kubelet that disoatcher and learned services are not useable now
			rm.logger.Info("Resource Controller has received shutdown message, withdraw resource advertisements.")
			d.Send(&pluginapi.ListAndWatchResponse{
				Devices: []*pluginapi.Device{}})
			return nil
		case <-rm.updateCh:
			// Received a notification of a change in advertised services
			d.Send(&pluginapi.ListAndWatchResponse{Devices: rm.buildDeviceList(pluginapi.Healthy)})
		}
	}
}

// Allocate which return list of devices.
func (rm *resourceManager) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		response := pluginapi.ContainerAllocateResponse{
			Devices: []*pluginapi.DeviceSpec{},
			Envs: map[string]string{
				"key": "dedi.io/dispatcher",
			},
		}
		for _, id := range req.DevicesIDs {
			deviceSpec := pluginapi.DeviceSpec{}
			deviceSpec.HostPath = id
			deviceSpec.ContainerPath = id
			deviceSpec.Permissions = "rw"
			response.Devices = append(response.Devices, &deviceSpec)
		}
		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}
	return &responses, nil
}

// register attempts to register Device Plugin with kubelet
func register(resource string, socket string, logger *zap.SugaredLogger) error {
	logger.Infof("Initiating registration with kubelet...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := tools.Dial(ctx, pluginapi.KubeletSocket)
	if err != nil {
		return err
	}
	defer conn.Close()
	logger.Infof("Registration dial succeeded...")
	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(socket),
		ResourceName: resource,
	}
	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	logger.Infof("Registration succeeded...")
	return nil
}
