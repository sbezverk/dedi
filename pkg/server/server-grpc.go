package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	api "github.com/sbezverk/dedi/pkg/apis/dispatcher"
	"github.com/sbezverk/dedi/pkg/tools"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	tempSocketBase = "/var/lib/dispatch"
	numberFDInMsg  = 1
)

var (
	sendTimeout        = 120 + time.Second
	recvTimeout        = 120 + time.Second
	socketReadyTimeout = 120 * time.Second
	retryInterval      = 1 * time.Second
)

type descriptor struct {
	socketControlMessage []byte
	sync.RWMutex
	used bool
}

func (d *descriptor) allocateSocket() {
	d.Lock()
	d.used = true
	d.Unlock()
}

func (d *descriptor) releaseSocket() {
	d.Lock()
	d.used = false
	d.Unlock()
}

// services is a map of all registered servicew and corresponding descriptors
type services map[string][]*descriptor

// Dispatcher is an interface to interact with dispatcher gRPC service
type Dispatcher interface {
	Connect(ctx context.Context, in *api.ConnectMsg) (*api.ReplyMsg, error)
	Listen(ctx context.Context, in *api.ListenMsg) (*api.ReplyMsg, error)
	Shutdown()
	Run() error
}

// NewDispatcher returns a new instance of a dispatcher service
func NewDispatcher(dispatcherSocket string, logger *zap.SugaredLogger, updateCh chan struct{}) (Dispatcher, error) {
	var err error
	svcs := services{}
	d := dispatcher{
		logger:   logger,
		stopCh:   make(chan struct{}),
		updateCh: updateCh,
	}
	d.services = svcs
	// Preparing to start Dispatcher gRPC server
	if err = tools.SocketCleanup(dispatcherSocket); err != nil {
		return nil, fmt.Errorf("Failed to cleaup stale socket with error: %+v", err)
	}
	// Setting up gRPC server
	d.listener, err = net.Listen("unix", dispatcherSocket)
	if err != nil {
		return nil, fmt.Errorf("Failed to setup listener with error: %+v", err)
	}
	d.server = grpc.NewServer([]grpc.ServerOption{}...)
	// Attaching Dispatcher API
	api.RegisterDispatcherServer(d.server, &d)

	return &d, nil
}

type dispatcher struct {
	server   *grpc.Server
	listener net.Listener
	stopCh   chan struct{}
	updateCh chan struct{}
	logger   *zap.SugaredLogger
	sync.RWMutex
	services
}

// Shutdown shuts down dispatcher
func (d *dispatcher) Shutdown() {
	// TODO Add shutdown logic
	d.server.Stop()
}

// Run start Disaptcher's gRPC server
func (d *dispatcher) Run() error {
	if err := d.server.Serve(d.listener); err != nil {
		return err
	}
	return nil
}

// Connect implements Connect method, this method is called when a pod wants to obtain
// a descriptor of a service to communicate with.
func (d *dispatcher) Connect(ctx context.Context, in *api.ConnectMsg) (*api.ReplyMsg, error) {
	out := new(api.ReplyMsg)
	// Check if requested service has already been registered
	descriptors, ok := d.services[in.SvcUuid]
	if !ok {
		return notAvailable(in, out)
	}
	// Need to select and mark inuse true a service entry
	for _, descr := range descriptors {
		if !descr.used {
			descr.allocateSocket()
			// Make temp file for SendMsg/RecvMsg communication
			uid := uuid.New().String()
			out.Socket = path.Join(tempSocketBase, in.SvcUuid+"-"+uid[len(uid)-8:])
			// Starting go routine to communicate with Connect
			// requested pod to pass FD and rights
			go d.sendDescr(in.SvcUuid, descr, out.Socket)
			return out, nil
		}
	}
	return notAvailable(in, out)
}

// Listen implements Listen method, this method is called when a pod wants to offer a service
// and to provide a descriptor for connecting to this service.
func (d *dispatcher) Listen(ctx context.Context, in *api.ListenMsg) (*api.ReplyMsg, error) {
	d.logger.Infof("Service advertisement for service: %s from pod: %s", in.SvcUuid, in.PodUuid)
	out := new(api.ReplyMsg)
	// Check if it is first instance of a service, if not instantiate it
	_, ok := d.services[in.SvcUuid]
	if !ok {
		d.Lock()
		d.services[in.SvcUuid] = make([]*descriptor, 0)
		d.Unlock()
	}
	out.PodUuid = in.PodUuid
	out.SvcUuid = in.SvcUuid
	uid := uuid.New().String()
	out.Socket = path.Join(tempSocketBase, in.SvcUuid+"-"+uid[len(uid)-8:])
	// Starting go routine to communicate with Service pod to
	// get descriptor information.
	go d.recvDescr(in.SvcUuid, out.Socket)
	return out, nil
}

func (d *dispatcher) recvDescr(svcID, socket string) {
	// Sanity cleanup of a possible stale socket files
	if err := tools.SocketCleanup(socket); err != nil {
		d.logger.Errorf("recvDescr: Service: %s error: %+v", svcID, err)
		return
	}
	sd, err := openSocket(socket)
	if err != nil {
		d.logger.Errorf("recvDescr: Service: %s error: %+v", svcID, err)
		return
	}
	defer func() {
		// Closing Unix Domain Socket used to receive Descriptor
		syscall.Close(sd)
		// Removing socket file
		os.Remove(socket)
	}()
	nd, err := recvMsg(sd)
	if err != nil {
		d.logger.Errorf("recvDescr: Service: %s error: %+v", svcID, err)
		return
	}
	newDescr := descriptor{
		socketControlMessage: nd,
		used:                 false,
	}
	// Updating services list with new instance
	d.Lock()
	defer d.Unlock()
	d.services[svcID] = append(d.services[svcID], &newDescr)
	d.logger.Infof("Service: %s has been registered", svcID)
	d.logger.Infof("Number of services in the store: %d", len(d.services))
}

func (d *dispatcher) sendDescr(svcID string, descr *descriptor, socket string) {
	// Check and Wait for Socket readiness
	if err := tools.CheckSocketReadiness(socket); err != nil {
		// By some unknown reason, the client has not opened the socket
		// within a timeout defined in socketReadyTimeout
		// Marking socket as not used and killing go routine.
		descr.releaseSocket()
		d.logger.Errorf("sendDescr: Service: %s error: %+v", svcID, err)
		return
	}
	if err := sendMsg(socket, descr.socketControlMessage); err != nil {
		// Failed to send descriptor information to the client.
		// Marking socket as not used and killing go routine.
		descr.releaseSocket()
		d.logger.Errorf("sendDescr: Service: %s error: %+v", svcID, err)
		return
	}
	// Send message succeeded, removing used descriptor
	// from listening services.
	d.removeDescr(svcID, descr)
	d.logger.Infof("Descriptor for Service : %s has been used", svcID)
	d.logger.Infof("Number of Descriptors for Service: %s left: %d", svcID, len(d.services[svcID]))
}

// removeDescr removes used descriptor from the descripts slice
func (d *dispatcher) removeDescr(svc string, descr *descriptor) {
	d.Lock()
	defer d.Unlock()
	sds := d.services[svc]
	for i, v := range sds {
		if v == descr {
			d.services[svc] = sds[:i]
			d.services[svc] = append(d.services[svc], sds[i+1:]...)
			break
		}
	}
}
