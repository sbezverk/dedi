package tools

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
)

var (
	sendTimeout        = 120 + time.Second
	recvTimeout        = 120 + time.Second
	socketReadyTimeout = 120 * time.Second
	retryInterval      = 1 * time.Second
)

// SocketCleanup checks the presense of old unix socket and if finds it, deletes it.
func SocketCleanup(listenEndpoint string) error {
	fi, err := os.Stat(listenEndpoint)
	if err == nil && (fi.Mode()&os.ModeSocket) != 0 {
		if err := os.Remove(listenEndpoint); err != nil {
			return fmt.Errorf("cannot remove listen endpoint %s with error: %+v", listenEndpoint, err)
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failure stat of socket file %s with error: %+v", listenEndpoint, err)
	}
	return nil
}

// CheckSocketReadiness check if the socket is ready to connect to
// it times ouut after the timeout defined in socketReadyTimeout
func CheckSocketReadiness(socket string) error {
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

// Dial attempts to dial gRPC endpoint and return client connection
func Dial(ctx context.Context, unixSocketPath string) (*grpc.ClientConn, error) {
	c, err := grpc.DialContext(ctx, "unix://"+unixSocketPath, grpc.WithInsecure(), grpc.WithBlock())
	return c, err
}
