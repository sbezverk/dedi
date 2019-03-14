package server

import (
	"context"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"

	api "github.com/sbezverk/memif2memif/pkg/apis/dispatcher"

	"go.uber.org/zap"
)

const (
	tempSocketBase = "/tmp"
)

var (
	sendTimeout        = 120 + time.Second
	socketReadyTimeout = 120 * time.Second
	retryInterval      = 1 * time.Second
)

type filedDescriptor struct {
	socketControlMessage []byte
	sync.RWMutex
	used bool
}

func (f *filedDescriptor) allocateSocket() {
	f.Lock()
	f.used = true
	f.Unlock()
}

func (f *filedDescriptor) releaseSocket() {
	f.Lock()
	f.used = false
	f.Unlock()
}

type service map[string]*filedDescriptor

// first key is svc id
type services map[string]service

type Dispatcher interface {
	Connect(ctx context.Context, in *api.ConnectMsg) (*api.ReplyMsg, error)
	Listen(ctx context.Context, in *api.ListenMsg) (*api.ReplyMsg, error)
}

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

func (d *dispatcher) Connect(ctx context.Context, in *api.ConnectMsg) (*api.ReplyMsg, error) {
	out := new(api.ReplyMsg)
	// Check if requested service has already been registered
	svc, ok := d.services[in.SvcUuid]
	if !ok {
		return notAvailable(in, out)
	}
	// Need to select and mark inuse true a service entry
	found := false
	for pod, fd := range svc {
		if !fd.used {
			fd.allocateSocket()
			found = true
			// Make temp file for SendMsg/RecvMsg communication
			uid := uuid.New().String()
			out.Socket = path.Join(tempSocketBase, in.SvcUuid+"-"+uid[len(uid)-8:])
			// Starting go routine to communicate with Connect
			// requested pod to pass FD and rights
			go d.sendFD(in.SvcUuid, pod, out.Socket)
		}
	}
	if !found {
		return notAvailable(in, out)
	}
	return out, nil
}

func (d *dispatcher) Listen(ctx context.Context, in *api.ListenMsg) (*api.ReplyMsg, error) {
	out := new(api.ReplyMsg)

	return out, nil
}

func (d *dispatcher) sendFD(svc, pod, socket string) {
	fd := d.services[svc][pod]
	// Check and Wait for Socket readiness
	if err := checkSocketReadiness(socket); err != nil {
		// By some unknown reason, the client has not opened the socket
		// within a timeout defined in socketReadyTimeout
		// Marking socket as not used and killing go routine.
		fd.releaseSocket()
		d.logger.Errorf("Service: %s Pod: %s error: %+v", svc, pod, err)
		return
	}
	if err := sendMsg(socket, fd.socketControlMessage); err != nil {
		// Failed to send FD information to the client.
		// Marking socket as not used and killing go routine.
		fd.releaseSocket()
		d.logger.Errorf("Service: %s Pod: %s error: %+v", svc, pod, err)
		return
	}
	// Send message succeeded, removing used svc/pod file descriptor
	// from listening services
	d.Lock()
	defer d.Unlock()
	delete(d.services[svc], pod)
}
