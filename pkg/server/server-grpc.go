package server

import (
	"context"

	"github.com/sbezverk/memif2memif/pkg/apis/dispatcher"
	"github.com/sbezverk/memif2memif/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.uber.org/zap"
)

// MemifDispatcher defines interface used to access MemifDispatcher data
type MemifDispatcher interface {
	Connect(ctx context.Context, in *dispatcher.ConnectMsg) (*dispatcher.SocketMsg, error)
	Listen(ctx context.Context, in *dispatcher.ListenMsg) (*dispatcher.SocketMsg, error)
}

// NewMemifDispatcher returns new instance of a memif dispatcher
func NewMemifDispatcher(logger *zap.SugaredLogger) MemifDispatcher {
	clients := memifDispatcher{
		logger: logger,
	}
	clients.Clients.Clients = make(map[types.ID]*types.Client)
	return &clients
}

// memifDispatcher internal struct for the dispatcher, storing clients with its sockets.
type memifDispatcher struct {
	types.Clients
	logger *zap.SugaredLogger
}

// Connect is called when a client pod wants to get a memif socket to connect to a memif enabled pod in
// listening mode.
func (m *memifDispatcher) Connect(ctx context.Context, in *dispatcher.ConnectMsg) (*dispatcher.SocketMsg, error) {
	m.logger.Infof("Connect request from pod: %s/%s", in.SrcId.PodNamespace, in.SrcId.PodName)
	out := new(dispatcher.SocketMsg)
	return out, status.Errorf(codes.Unavailable, "Connect has not been yet implemented")
}

// Listen is called when a clinet pod wants to get a memif socket to listen for incoming memif connections.
func (m *memifDispatcher) Listen(ctx context.Context, in *dispatcher.ListenMsg) (*dispatcher.SocketMsg, error) {
	m.logger.Infof("Listen request from pod: %s/%s", in.ListenerId.PodNamespace, in.ListenerId.PodName)
	out := new(dispatcher.SocketMsg)
	out.PodId = in.ListenerId

	sock, err := m.listener(in)
	if err != nil {
		out.Success = false
		return out, status.Errorf(codes.Aborted, "Listen failed to obtain a socket with error: %+v", err)
	}
	out.Fd = int64(sock)
	out.Success = true

	return out, nil
}
