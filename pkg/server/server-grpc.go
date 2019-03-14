package server

import (
	"context"

	api "github.com/sbezverk/memif2memif/pkg/apis/dispatcher"

	"go.uber.org/zap"
)

type Dispatcher interface {
	Connect(ctx context.Context, in *api.ConnectMsg) (*api.ReplyMsg, error)
	Listen(ctx context.Context, in *api.ListenMsg) (*api.ReplyMsg, error)
}

func NewDispatcher(logger *zap.SugaredLogger) Dispatcher {
	clients := dispatcher{
		logger: logger,
	}
	return &clients
}

type dispatcher struct {
	logger *zap.SugaredLogger
}

func (m *dispatcher) Connect(ctx context.Context, in *api.ConnectMsg) (*api.ReplyMsg, error) {
	out := new(api.ReplyMsg)

	return out, nil
}

func (m *dispatcher) Listen(ctx context.Context, in *api.ListenMsg) (*api.ReplyMsg, error) {
	out := new(api.ReplyMsg)

	return out, nil
}
