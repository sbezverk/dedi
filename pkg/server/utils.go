package server

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/sbezverk/memif2memif/pkg/apis/dispatcher"
)

func notAvailable(in *api.ConnectMsg, out *api.ReplyMsg) (*api.ReplyMsg, error) {
	out.Error = api.ERR_SVC_NOT_AVAILABLE
	out.PodUuid = in.PodUuid
	out.SvcUuid = in.SvcUuid
	return out, status.Errorf(codes.Unavailable, "Connect failed, requested service %s is not available", in.SvcUuid)
}

func checkSocketReadiness(socket string) error {
	ticker := time.NewTicker(retryInterval)
	timeOut := time.NewTimer(socketReadyTimeout)
	for {
		fi, err := os.Stat(socket)
		if err == nil && (fi.Mode()&os.ModeSocket) != 0 {
			// File exists and it is a socket, all good..
			return nil
		}
		select {
		case <-ticker.C:
			continue
		case <-timeOut.C:
			return fmt.Errorf("Timed out waiting for socket: %s to become ready", socket)
		}
	}
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
