package server

import (
	"github.com/sbezverk/memif2memif/pkg/apis/dispatcher"
	"github.com/sbezverk/memif2memif/pkg/types"
	"github.com/sbezverk/memif2memif/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (m *memifDispatcher) listener(in *dispatcher.ListenMsg) (types.Socket, error) {
	var client *types.Client
	// Check if this listener is already in the list, if not need to add it.
	id := types.ID{
		Name:      in.ListenerId.PodName,
		Namespace: in.ListenerId.PodNamespace,
	}
	client, ok := m.Clients.Clients[id]
	if !ok {
		client = m.addClient(id, in.MaxConnections)
	}
	m.logger.Infof("Listening client: %+v", client.Listen)

	// Check if number of listen requests does not exceed maximum supported connections
	if int32(len(client.Listen)) >= client.MaxConnections {
		return types.Socket(0), status.Errorf(codes.ResourceExhausted, "Exceeded number of supported Listen connections")
	}
	sock, err := utils.AllocateSocket()
	if err != nil {
		return types.Socket(0), status.Errorf(codes.Aborted, "Failed to allocate a socket with error: %+v", err)
	}

	return types.Socket(sock), status.Errorf(codes.Unavailable, "Listen has not been yet implemented")
}

// add adds a new client and initializes its Listen and Connect structures
func (m *memifDispatcher) addClient(id types.ID, maxConnection int32) *types.Client {
	client := types.Client{
		MaxConnections: maxConnection,
		Listen:         make(map[types.Socket]types.State),
		Connect:        make(map[types.Socket]types.State),
	}
	m.Clients.Lock()
	defer m.Clients.Unlock()
	m.Clients.Clients[id] = &client

	return &client
}
