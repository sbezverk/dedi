package controller

import (
	// "encoding/json"
	// "fmt"
	"net"
	"os"
	"path"
	"time"
	// "strings"
	// "sync"
	"github.com/sbezverk/memif2memif/pkg/tools"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	serverBasePath = pluginapi.DevicePluginPath
)

type resourceController struct {
	socket   string
	server   *grpc.Server
	stopCh   chan struct{}
	updateCh chan struct{}
	logger   *zap.SugaredLogger
}

// ResourceController is interface to access resourceController resources
type ResourceController interface {
	Run() error
	Shutdown()
}

// NewResourceController creates an instance of a new resourceCOntroller and returns its interface
func NewResourceController(logger *zap.SugaredLogger, updateCh chan struct{}) ResourceController {
	return &resourceController{
		logger:   logger,
		stopCh:   make(chan struct{}),
		updateCh: updateCh,
	}
}

func (rs *resourceController) Run() error {

	return nil
}

func (rs *resourceController) Shutdown() {
	// TODO add shutdown logic
}

func (rs *resourceController) startDevicePlugin() {

	rs.socket = path.Join(serverBasePath, "dispatch-resource-controller.sock")

	// starting gRPC server for kubelet's Allocate and ListAndWatch calls
	rs.startServer()

	rs.register()

}

func (rs *resourceController) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (rs *resourceController) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (rs *resourceController) startServer() error {
	err := rs.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", rs.socket)
	if err != nil {
		return err
	}

	rs.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(rs.server, rs)

	go rs.server.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := tools.Dial(ctx, rs.socket)
	if err != nil {
		return err
	}
	conn.Close()

	return nil
}

func (rs *resourceController) cleanup() error {
	if err := os.Remove(rs.socket); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (rs *resourceController) register() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := tools.Dial(ctx, pluginapi.KubeletSocket)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(rs.socket),
		ResourceName: "dispatch-resource-controller",
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

func (rs *resourceController) buildDeviceList(health string) []*pluginapi.Device {
	deviceList := []*pluginapi.Device{}

	device := pluginapi.Device{}
	device.Health = health
	deviceList = append(deviceList, &device)

	return deviceList
}

// ListAndWatch converts VFs into device and list them
func (rs *resourceController) ListAndWatch(e *pluginapi.Empty, d pluginapi.DevicePlugin_ListAndWatchServer) error {
	d.Send(&pluginapi.ListAndWatchResponse{Devices: rs.buildDeviceList(pluginapi.Healthy)})
	for {
		select {
		case <-rs.stopCh:
			// Informing kubelet that VFs which belong to network service are not useable now
			d.Send(&pluginapi.ListAndWatchResponse{
				Devices: []*pluginapi.Device{}})
			return nil
		case <-rs.updateCh:
			// Received a notification of a change in VFs resending updated list to kubelet
			d.Send(&pluginapi.ListAndWatchResponse{Devices: rs.buildDeviceList(pluginapi.Healthy)})
		}
	}
}

// Allocate which return list of devices.
func (rs *resourceController) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {

	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		response := pluginapi.ContainerAllocateResponse{
			Devices: []*pluginapi.DeviceSpec{},
			Envs:    map[string]string{
				// key: path.Join(containerConfigFilePath, configFileName),
			},
			Mounts: []*pluginapi.Mount{
				&pluginapi.Mount{
					ContainerPath: "", // path.Join(containerConfigFilePath, configFileName),
					HostPath:      "", // configFile.Name(),
					ReadOnly:      true,
				},
			},
		}
		for _, id := range req.DevicesIDs {
			deviceSpec := pluginapi.DeviceSpec{}
			deviceSpec.HostPath = id
			deviceSpec.ContainerPath = id
			deviceSpec.Permissions = "rw"
			response.Devices = append(response.Devices, &deviceSpec)
			// Getting vfio device specific specifications and storing it in the slice. The slice
			// will be marshalled into json and passed to requesting POD as a mount.
		}
		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}
	return &responses, nil
}
