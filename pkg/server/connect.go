package server

import (
	"github.com/sbezverk/memif2memif/pkg/apis/dispatcher"
	"github.com/sbezverk/memif2memif/pkg/types"
	"github.com/sbezverk/memif2memif/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (m *memifDispatcher) connecter(in *dispatcher.ConnectMsg) (types.Socket, error) {
	var client *types.Client
	// Check if this listener is already in the list, if not need to add it.
	id := types.ID{
		Name:      in.SrcId.PodName,
		Namespace: in.SrcId.PodNamespace,
	}
	client, ok := m.Clients.Clients[id]
	if !ok {
		client = m.addClient(id, 0)
	}
	m.logger.Infof("Connecting client: %+v", client.Listen)

	// Check if number of listen requests does not exceed maximum supported connections
	if int32(len(client.Listen)) >= client.MaxConnections {
		return types.Socket(0), status.Errorf(codes.ResourceExhausted, "Exceeded number of supported Listen connections")
	}
	sock, err := utils.AllocateSocket(id)
	if err != nil {
		return types.Socket(0), status.Errorf(codes.Aborted, "Failed to allocate a socket with error: %+v", err)
	}
	client.LockListen()
	defer client.UnlockListen()
	// TODO: Need to figure out cleanup process
	client.Listen[types.Socket(sock)] = types.Allocated

	return types.Socket(sock), nil
}
