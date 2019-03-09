package server

import (
	"context"

	"github.com/sbezverk/memif2memif/pkg/apis/dispatcher"
	"github.com/sbezverk/memif2memif/pkg/types"

	"go.uber.org/zap"
)

// MemifDispatcher defines interface used to access MemifDispatcher data
type MemifDispatcher interface {
	Socket(ctx context.Context, in *dispatcher.SocketMsg) (*dispatcher.ReplyMsg, error)
	Intro(ctx context.Context, in *dispatcher.IntroMsg) (*dispatcher.ReplyMsg, error)
	Connect(ctx context.Context, in *dispatcher.ConnectMsg) (*dispatcher.ReplyMsg, error)
	Listen(ctx context.Context, in *dispatcher.ListenMsg) (*dispatcher.ReplyMsg, error)
}

// NewMemifDispatcher returns new instance of a memif dispatcher
func NewMemifDispatcher(logger *zap.SugaredLogger) MemifDispatcher {
	clients := memifDispatcher{}
	clients.Clients.Client = make(map[types.ID]map[types.Socket]types.State)
	return &clients
}

type memifDispatcher struct {
	types.Clients
	logger *zap.SugaredLogger
}

func (m *memifDispatcher) Socket(ctx context.Context, in *dispatcher.SocketMsg) (*dispatcher.ReplyMsg, error) {
	out := new(dispatcher.ReplyMsg)
	return out, nil
}

func (m *memifDispatcher) Intro(ctx context.Context, in *dispatcher.IntroMsg) (*dispatcher.ReplyMsg, error) {
	out := new(dispatcher.ReplyMsg)
	return out, nil
}

func (m *memifDispatcher) Connect(ctx context.Context, in *dispatcher.ConnectMsg) (*dispatcher.ReplyMsg, error) {
	out := new(dispatcher.ReplyMsg)
	return out, nil
}

func (m *memifDispatcher) Listen(ctx context.Context, in *dispatcher.ListenMsg) (*dispatcher.ReplyMsg, error) {
	out := new(dispatcher.ReplyMsg)
	return out, nil
}
