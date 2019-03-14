package server

import (
	"context"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"

	api "github.com/sbezverk/memif2memif/pkg/apis/dispatcher"
	"github.com/sbezverk/memif2memif/pkg/tools"
	"go.uber.org/zap"
)

const (
	tempSocketBase = "/tmp"
	numberFDInMsg  = 1
)

var (
	sendTimeout        = 120 + time.Second
	recvTimeout        = 120 + time.Second
	socketReadyTimeout = 120 * time.Second
	retryInterval      = 1 * time.Second
)

type fileDescriptor struct {
	socketControlMessage []byte
	sync.RWMutex
	used bool
}

func (f *fileDescriptor) allocateSocket() {
	f.Lock()
	f.used = true
	f.Unlock()
}

func (f *fileDescriptor) releaseSocket() {
	f.Lock()
	f.used = false
	f.Unlock()
}

// service is a map of all pods offering a specific service
type service map[string]*fileDescriptor

// service is a map of all registered services
type services map[string]service

// Dispatcher is an interface to interact with dispatcher gRPC service
type Dispatcher interface {
	Connect(ctx context.Context, in *api.ConnectMsg) (*api.ReplyMsg, error)
	Listen(ctx context.Context, in *api.ListenMsg) (*api.ReplyMsg, error)
}

// NewDispatcher returns a new instance of a dispatcher service
func NewDispatcher(logger *zap.SugaredLogger) Dispatcher {
	svcs := services{}
	d := dispatcher{
		logger: logger,
	}
	d.services = svcs
	return &d
}

type dispatcher struct {
	logger *zap.SugaredLogger
	sync.RWMutex
	services
}

// Connect implements Connect method, this method is called when a pod wants to obtain
// a file descriptor of a service to communicate with.
func (d *dispatcher) Connect(ctx context.Context, in *api.ConnectMsg) (*api.ReplyMsg, error) {
	out := new(api.ReplyMsg)
	// Check if requested service has already been registered
	svc, ok := d.services[in.SvcUuid]
	if !ok {
		return notAvailable(in, out)
	}
	// Need to select and mark inuse true a service entry
	for pod, fd := range svc {
		if !fd.used {
			fd.allocateSocket()
			// Make temp file for SendMsg/RecvMsg communication
			uid := uuid.New().String()
			out.Socket = path.Join(tempSocketBase, in.SvcUuid+"-"+uid[len(uid)-8:])
			// Starting go routine to communicate with Connect
			// requested pod to pass FD and rights
			go d.sendFD(in.SvcUuid, pod, out.Socket)
			return out, nil
		}
	}
	return notAvailable(in, out)
}

// Listen implements Listen method, this method is called when a pod wants to offer a service
// and to provide a file descriptor for connecting to this service.
func (d *dispatcher) Listen(ctx context.Context, in *api.ListenMsg) (*api.ReplyMsg, error) {
	d.logger.Infof("Service advertisement for service: %s from pod: %s", in.SvcUuid, in.PodUuid)
	out := new(api.ReplyMsg)
	// Check if it is first instance of a service, if not instantiate it
	svc, ok := d.services[in.SvcUuid]
	if !ok {
		svc = make(service)
		svc[in.PodUuid] = &fileDescriptor{}
	}
	// check if there is already pod offerring this service, if not instantiate it
	fd, ok := svc[in.PodUuid]
	if !ok {
		fd = &fileDescriptor{}
		svc[in.PodUuid] = fd
	}

	out.PodUuid = in.PodUuid
	out.SvcUuid = in.SvcUuid
	uid := uuid.New().String()
	out.Socket = path.Join(tempSocketBase, in.SvcUuid+"-"+uid[len(uid)-8:])
	// Starting go routine to communicate with Service pod to
	// get file descriptor information
	go d.recvFD(svc, in.SvcUuid, in.PodUuid, out.Socket)

	return out, nil
}

func (d *dispatcher) recvFD(svc service, svcID, pod, socket string) {
	// Sanity cleanup of a possible stale socket files
	if err := tools.SocketCleanup(socket); err != nil {
		d.logger.Errorf("recvFD: Service: %s Pod: %s error: %+v", svc, pod, err)
		return
	}
	sd, err := openSocket(socket)
	if err != nil {
		d.logger.Errorf("recvFD: Service: %s Pod: %s error: %+v", svc, pod, err)
		return
	}
	fd, err := recvMsg(sd)
	if err != nil {
		d.logger.Errorf("recvFD: Service: %s Pod: %s error: %+v", svc, pod, err)
		return
	}
	svc[pod].socketControlMessage = fd
	svc[pod].used = false
	// Updating services list with new instance
	d.Lock()
	defer d.Unlock()
	d.services[svcID] = svc
	d.logger.Infof("Service: %s from pod: %s has been registered", svcID, pod)
	d.logger.Infof("Number of services in the store: %d", len(d.services))
}

func (d *dispatcher) sendFD(svc, pod, socket string) {
	fd := d.services[svc][pod]
	// Check and Wait for Socket readiness
	if err := tools.CheckSocketReadiness(socket); err != nil {
		// By some unknown reason, the client has not opened the socket
		// within a timeout defined in socketReadyTimeout
		// Marking socket as not used and killing go routine.
		fd.releaseSocket()
		d.logger.Errorf("sendFD: Service: %s Pod: %s error: %+v", svc, pod, err)
		return
	}
	if err := sendMsg(socket, fd.socketControlMessage); err != nil {
		// Failed to send FD information to the client.
		// Marking socket as not used and killing go routine.
		fd.releaseSocket()
		d.logger.Errorf("sendFD: Service: %s Pod: %s error: %+v", svc, pod, err)
		return
	}
	// Send message succeeded, removing used svc/pod file descriptor
	// from listening services
	d.Lock()
	defer d.Unlock()
	delete(d.services[svc], pod)
}
