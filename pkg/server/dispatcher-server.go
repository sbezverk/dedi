package server

import (
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
	msgSendInterval    = 5 * time.Second
)

// descriptor is struct storing informatiobn related to a specific service
type descriptor struct {
	socketControlMessage []byte
	maxConnections       int32
	requestCh            chan struct{}
	responseCh           chan bool
	releaseCh            chan struct{}
	sync.RWMutex
	usedConnections int32
}

// services is a map of all registered services, second key is pod/process id and corresponding descriptors
type services struct {
	sync.RWMutex
	store map[string]map[string]*descriptor
}

// Dispatcher is an interface to interact with dispatcher gRPC service
type Dispatcher interface {
	Shutdown()
	Run() error
}

// NewDispatcher returns a new instance of a dispatcher service
func NewDispatcher(dispatcherSocket string, logger *zap.SugaredLogger, updateCh chan string) (Dispatcher, error) {
	var err error

	d := dispatcher{
		logger:   logger,
		stopCh:   make(chan struct{}),
		updateCh: updateCh,
	}
	d.services.store = make(map[string]map[string]*descriptor)
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
	updateCh chan string
	logger   *zap.SugaredLogger
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
func (d *dispatcher) Connect(in *api.ConnectMsg, stream api.Dispatcher_ConnectServer) error {
	d.logger.Infof("Connect request for Service: %s from pod: %s", in.SvcUuid, in.PodUuid)

	ticker := time.NewTicker(msgSendInterval)
	// Finding required service loop
	// There are 2 possibilities to exit this loop:
	// 1  - Requested service already exists and it has available connections
	// 2 - Client decides to bail upon receiving ERR_SVC_NOT_AVAILABLE
	// Otherwise each msgSendInterval interval Services Store will be checked for required
	// service availablelity and requesting client will get  ERR_SVC_NOT_AVAILABLE again.
	var sd *descriptor
Loop:
	for {
		if _, ok := d.services.store[in.SvcUuid]; ok {
			d.logger.Infof("Connect: Service %s found", in.SvcUuid)
			for pod, desc := range d.services.store[in.SvcUuid] {
				desc.requestCh <- struct{}{}
				if <-desc.responseCh {
					d.logger.Infof("Connect: Found availavle descriptor for service: %s hosting pod: %s", in.SvcUuid, pod)
					sd = desc
					ticker.Stop()
					break Loop
				}
			}
			d.logger.Infof("Connect: Service %s found but no available connections", in.SvcUuid)

		} else {
			d.logger.Infof("Connect: Service %s not found", in.SvcUuid)
		}
		if err := stream.Send(&api.ReplyMsg{Error: api.ERR_SVC_NOT_AVAILABLE}); err != nil {
			d.logger.Warnf("Connect: Pod: %s requested Service: %s terminated connection with error: %+v, exiting...", in.PodUuid, in.SvcUuid, err)
			return err
		}
		d.logger.Infof("Connect: Sent client SVC_NOT_AVAILABLE")
		select {
		case <-ticker.C:
			continue
		}
	}
	// Since the descriptor has been selected and it is available, sending info to the requesting client
	defer func() {
		// If this function exists, releasing used desriptor
		sd.releaseCh <- struct{}{}
	}()
	out := new(api.ReplyMsg)
	uid := uuid.New().String()
	out.Socket = path.Join(tempSocketBase, in.SvcUuid+"-"+uid[len(uid)-8:])
	out.Error = api.ERR_NO_ERROR
	if err := stream.Send(out); err != nil {
		d.logger.Warnf("Connect: Failed to send Pod: %s Service: %s Socket message with error: %+v, exiting...", in.PodUuid, in.SvcUuid, err)
		return err
	}
	// Starting go routine to communicate with Connect
	// requested pod to pass FD and rights
	if err := d.sendDescr(in.SvcUuid, sd, out.Socket); err != nil {
		d.logger.Warnf("Connect: Failed to send descriptor to Pod: %s for Service: %s with error: %+v, exiting...", in.PodUuid, in.SvcUuid, err)
		return err
	}

	// Connection liveness timer
	ticker = time.NewTicker(msgSendInterval)
	for {
		if err := stream.Send(&api.ReplyMsg{Error: api.ERR_KEEPALIVE}); err != nil {
			d.logger.Warnf("Connect: Connection with %s has been lost with error: %+v", in.PodUuid, err)
			return err
		}
		select {
		case <-ticker.C:
			continue
		}
	}
}

// Listen implements Listen method, this method is called when a pod wants to offer a service
// and to provide a descriptor for connecting to this service.
func (d *dispatcher) Listen(in *api.ListenMsg, stream api.Dispatcher_ListenServer) error {
	d.logger.Infof("Service advertisement for service: %s from pod: %s", in.SvcUuid, in.PodUuid)
	out := new(api.ReplyMsg)
	// Check if it is first instance of a service, if not instantiate it
	_, ok := d.services.store[in.SvcUuid]
	if !ok {
		d.services.Lock()
		d.services.store[in.SvcUuid] = make(map[string]*descriptor, 0)
		d.services.Unlock()
	}
	out.PodUuid = in.PodUuid
	out.SvcUuid = in.SvcUuid
	uid := uuid.New().String()
	out.Socket = path.Join(tempSocketBase, in.SvcUuid+"-"+uid[len(uid)-8:])
	if err := stream.Send(out); err != nil {
		d.logger.Errorf("Listen: failed to send socket info with error: %+v", err)
		return err
	}
	fd, err := d.recvDescr(in.SvcUuid, out.Socket)
	if err != nil {
		d.logger.Errorf("Listen: failed to receive the descriptor for Service: %swith error: %+v", in.SvcUuid, err)
		return err
	}
	nd := descriptor{
		socketControlMessage: fd,
		maxConnections:       in.MaxConnections,
		usedConnections:      int32(0),
		requestCh:            make(chan struct{}),
		responseCh:           make(chan bool),
		releaseCh:            make(chan struct{}),
	}
	d.services.Lock()
	d.services.store[in.SvcUuid][in.PodUuid] = &nd
	d.services.Unlock()
	d.logger.Infof("Listen: Number of pods offering service: %s - %d", in.SvcUuid, len(d.services.store[in.SvcUuid]))

	// Connection liveness timer
	ticker := time.NewTicker(msgSendInterval)
	for {
		if err := stream.Send(&api.ReplyMsg{Error: api.ERR_KEEPALIVE}); err != nil {
			d.logger.Warnf("Listen: Connection with %s has been lost with error: %+v, removing [%s/%s] from the store.",
				in.PodUuid, err, in.PodUuid, in.SvcUuid)
			d.services.Lock()
			defer d.services.Unlock()
			delete(d.services.store[in.SvcUuid], in.PodUuid)
			return err
		}
		select {
		case <-ticker.C:
			continue
			//
		case <-nd.requestCh:
			d.logger.Infof("Listen: Message on request channel usedConnection: %d maxConnections: %d", nd.usedConnections, nd.maxConnections)
			if nd.usedConnections >= nd.maxConnections {
				nd.responseCh <- false
			} else {
				nd.Lock()
				nd.usedConnections++
				nd.Unlock()
				nd.responseCh <- true
			}
			// When Descriptor consumer termoinates, its go routine sends a message over descriptor relaseCh
			// upon receiving such message decrement number of used connections.
		case <-nd.releaseCh:
			nd.Lock()
			nd.usedConnections--
			if nd.usedConnections < 0 {
				nd.usedConnections = 0
			}
			nd.Unlock()
		}
	}
}

func (d *dispatcher) recvDescr(svcID, socket string) ([]byte, error) {
	// Sanity cleanup of a possible stale socket files
	if err := tools.SocketCleanup(socket); err != nil {
		d.logger.Errorf("recvDescr: Service: %s error: %+v", svcID, err)
		return nil, err
	}
	sd, err := openSocket(socket)
	if err != nil {
		d.logger.Errorf("recvDescr: Service: %s error: %+v", svcID, err)
		return nil, err
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
		return nil, err
	}
	return nd, nil
}

func (d *dispatcher) sendDescr(svcID string, descr *descriptor, socket string) error {
	// Check and Wait for Socket readiness
	if err := tools.CheckSocketReadiness(socket); err != nil {
		// By some unknown reason, the client has not opened the socket
		// within a timeout defined in socketReadyTimeout
		// Marking socket as not used and killing go routine.
		d.logger.Errorf("sendDescr: Service: %s error: %+v", svcID, err)
		return err
	}
	if err := sendMsg(socket, descr.socketControlMessage); err != nil {
		// Failed to send descriptor information to the client.
		// Marking socket as not used and killing go routine.
		d.logger.Errorf("sendDescr: Service: %s error: %+v", svcID, err)
		return err
	}
	return nil
}
