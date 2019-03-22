package server

import (
	"fmt"
	"net"
	"syscall"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/sbezverk/dedi/pkg/apis/dispatcher"
	"github.com/sbezverk/dedi/pkg/types"
)

func notAvailable(in *api.ConnectMsg, out *api.ReplyMsg) (*api.ReplyMsg, error) {
	out.Error = api.ERR_SVC_NOT_AVAILABLE
	out.PodUuid = in.PodUuid
	out.SvcUuid = in.SvcUuid
	return out, status.Errorf(codes.Unavailable, "Connect failed, requested service %s is not available", in.SvcUuid)
}

func sendMsg(socket string, socketControlMessage []byte) error {
	uc, err := net.DialUnix("unixgram", nil, &net.UnixAddr{
		Name: socket,
		Net:  "unixgram",
	})
	if err != nil {
		return fmt.Errorf("Failed to Dial socket %s with error: %+v", socket, err)

	}
	f, _ := uc.File()
	fd := f.Fd()
	if err := syscall.Sendmsg(int(fd), nil, socketControlMessage, nil, 0); err != nil {
		return fmt.Errorf("Failed to SendMsg with error: %+v", err)
	}
	return nil
}

func openSocket(socket string) (int, error) {
	uc, err := net.ListenUnixgram("unixgram", &net.UnixAddr{
		Name: socket,
		Net:  "unixgram",
	})
	if err != nil {
		return 0, fmt.Errorf("Failed to listen on socket %s with error: %+v", socket, err)

	}
	// Limiting recvMsg wait time to timeout defined by recvTimeout
	uc.SetDeadline(time.Now().Add(recvTimeout))
	f, _ := uc.File()
	fd := f.Fd()

	return int(fd), nil
}

func recvMsg(sd int) ([]byte, error) {
	buf := make([]byte, syscall.CmsgSpace(numberFDInMsg*4))

	_, _, _, _, err := syscall.Recvmsg(sd, nil, buf, 0)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func sendUpdate(updateCh chan types.UpdateOp, op types.Operation, svcID string, maxConnections, availableConnections int32) {
	update := types.UpdateOp{
		ServiceID: svcID,
	}
	switch op {
	case types.Add:
		update.Op = types.Add
		update.MaxConnections = maxConnections
	case types.Delete:
		update.Op = types.Delete
	case types.Update:
		update.Op = types.Update
		update.AvailableConnections = availableConnections
	}
	updateCh <- update
}
