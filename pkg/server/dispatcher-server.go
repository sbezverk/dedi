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
	"github.com/sbezverk/dedi/pkg/types"
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

// descriptor is struct storing informatiobn related to a specific service instance
type descriptor struct {
	socketControlMessage []byte
	maxConnections       int32
	//  requestCh is used to communicate a request from client for available descriptor
	requestCh chan struct{}
	// responseCh is used to indicate success or failure of the descriptor request
	responseCh chan bool
	// releaseCh is used to indicate when a client stopped using previsously allocated descriptor
	releaseCh chan struct{}
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
func NewDispatcher(dispatcherSocket string, logger *zap.SugaredLogger, updateCh chan types.UpdateOp) (Dispatcher, error) {
	var err error

	d := dispatcher{
		logger:   logger,
		stopCh:   make(chan struct{}),
		updateCh: updateCh,
	}
	// Instantiating a store, which is a map with primary key of Service ID, and secondary key
	// process or pod id. Specific service could be offered by multiple pods/processes as long as
	// their IDs are unique.
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
	// updateCh is used to communicate with controller manager to
	// advertise availavble services via Device Plugin API
	updateCh chan types.UpdateOp
	logger   *zap.SugaredLogger
	// Services storage
	services
}

// Shutdown shuts down dispatcher
func (d *dispatcher) Shutdown() {
	// Shutting down gRPC server will force all currently active Listen/Connect
	// go routine to fail on send and exit.
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
	// Finding required service/descriptor loop
	// The code will loop and send  ERR_SVC_NOT_AVAILABLE to the client until, the requested service
	// with available descripto is found, or a client that requested a service closes its side of the connection.
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
					//  Service and Descriptor are found, can break out of the loop
					break Loop
				}
			}
			d.logger.Infof("Connect: Service %s found but no available connections", in.SvcUuid)

		} else {
			d.logger.Infof("Connect: Service %s not found", in.SvcUuid)
		}
		// Sending client ERR_SVC_NOT_AVAILABLE message and wait for another cycle
		if err := stream.Send(&api.ReplyMsg{Error: api.ERR_SVC_NOT_AVAILABLE}); err != nil {
			d.logger.Warnf("Connect: Pod: %s requested Service: %s terminated connection with error: %+v, exiting...", in.PodUuid, in.SvcUuid, err)
			return err
		}
		select {
		case <-ticker.C:
			continue
		}
	}

	defer func() {
		// If Connect function exists by any reasons, sending release message to free up used descriptor
		sd.releaseCh <- struct{}{}
	}()
	// Sending socket path to the client
	out := new(api.ReplyMsg)
	uid := uuid.New().String()
	out.Socket = path.Join(tempSocketBase, in.SvcUuid+"-"+uid[len(uid)-8:])
	out.Error = api.ERR_NO_ERROR
	if err := stream.Send(out); err != nil {
		d.logger.Warnf("Connect: Failed to send Pod: %s Service: %s Socket message with error: %+v, exiting...", in.PodUuid, in.SvcUuid, err)
		return err
	}
	// Sending Descriptor over previosuly communicated socket
	if err := d.sendDescr(in.SvcUuid, sd, out.Socket); err != nil {
		d.logger.Warnf("Connect: Failed to send descriptor to Pod: %s for Service: %s with error: %+v, exiting...", in.PodUuid, in.SvcUuid, err)
		return err
	}

	// At this point the client received Descriptor and can communicate with it is service, the following for loop
	// checks liveness of the client and if the connection is dropped, this go routine is terminated.
	// Defered  call will free up used descriptor
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
	// Sending update to resource controller to announce new service availablility
	// only if it is a first pod hosting this service
	// otherwise just updating number of available connections
	if len(d.services.store[in.SvcUuid]) == 1 {
		sendUpdate(d.updateCh, types.Add, in.SvcUuid, in.MaxConnections)
	} else {
		sendUpdate(d.updateCh, types.Update, in.SvcUuid, getServiceAvailableConnections(d.services.store[in.SvcUuid]))
	}

	// Connection liveness timer
	ticker := time.NewTicker(msgSendInterval)
	for {
		if err := stream.Send(&api.ReplyMsg{Error: api.ERR_KEEPALIVE}); err != nil {
			d.logger.Warnf("Listen: Connection with %s has been lost with error: %+v, removing [%s/%s] from the store.",
				in.PodUuid, err, in.PodUuid, in.SvcUuid)
			d.services.Lock()
			defer d.services.Unlock()
			// Removing pod from the store
			delete(d.services.store[in.SvcUuid], in.PodUuid)
			// One of Service hosting pods is gone, sending update message to resource controller to stop
			// advertising if it was the last pod hosting the service
			// Otherwise just reducing number of avialble connections.
			if len(d.services.store[in.SvcUuid]) == 0 {
				// No moore pods left hosting the service, it is safe to delete the service
				sendUpdate(d.updateCh, types.Delete, in.SvcUuid, 0)
			} else {
				// There are still pods hosting the service, just update the number of avialble connections
				sendUpdate(d.updateCh, types.Update, in.SvcUuid, getServiceAvailableConnections(d.services.store[in.SvcUuid]))
			}
			return err
		}
		select {
		case <-ticker.C:
			continue
		case <-nd.requestCh:
			// Checking for limitss
			if nd.usedConnections >= nd.maxConnections {
				nd.responseCh <- false
			} else {
				nd.Lock()
				nd.usedConnections++
				nd.Unlock()
				nd.responseCh <- true
				sendUpdate(d.updateCh, types.Update, in.SvcUuid, getServiceAvailableConnections(d.services.store[in.SvcUuid]))
			}
		case <-nd.releaseCh:
			// When Descriptor consumer termoinates, its go routine sends a message over descriptor relaseCh
			// upon receiving such message decrement number of used connections.
			nd.Lock()
			nd.usedConnections--
			if nd.usedConnections < 0 {
				nd.usedConnections = 0
			}
			nd.Unlock()
			sendUpdate(d.updateCh, types.Update, in.SvcUuid, getServiceAvailableConnections(d.services.store[in.SvcUuid]))
		}
		d.logger.Infof("Listen: Service: %s with available connections: %d", in.SvcUuid, getServiceAvailableConnections(d.services.store[in.SvcUuid]))
	}
}

func (d *dispatcher) recvDescr(svcID, socket string) ([]byte, error) {
	// Sanity cleanup of a possible stale socket files
	if err := tools.SocketCleanup(socket); err != nil {
		d.logger.Errorf("recvDescr: Service: %s error: %+v", svcID, err)
		return nil, err
	}
	sd, err := OpenSocket(socket)
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
