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
	clients.Clients.Clients = make(map[types.ID]types.Client)
	return &clients
}

type memifDispatcher struct {
	types.Clients
	logger *zap.SugaredLogger
}

func (m *memifDispatcher) Connect(ctx context.Context, in *dispatcher.ConnectMsg) (*dispatcher.SocketMsg, error) {
	m.logger.Infof("Connect request from pod: %s/%s", in.SrcId.PodNamespace, in.SrcId.PodName)
	out := new(dispatcher.SocketMsg)
	return out, status.Errorf(codes.Unavailable, "Connect has not been yet implemented")
}

func (m *memifDispatcher) Listen(ctx context.Context, in *dispatcher.ListenMsg) (*dispatcher.SocketMsg, error) {
	m.logger.Infof("Listen request from pod: %s/%s", in.ListenerId.PodNamespace, in.ListenerId.PodName)
	out := new(dispatcher.SocketMsg)
	return out, status.Errorf(codes.Unavailable, "Connect has not been yet implemented")
}
