package main

import (
	"context"
	"flag"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/knative/pkg/signals"
	"github.com/sbezverk/memif2memif/pkg/apis/dispatcher"

	"go.uber.org/zap"
)

var (
	logger *zap.SugaredLogger
)

func init() {
	// Setting up logger
	l, err := zap.NewProduction()
	if err != nil {
		os.Exit(1)
	}
	logger = l.Sugar()
}

func main() {
	var err error
	flag.Parse()
	logger.Infof("Starting Connect...")
	clientConn, err := dial(context.Background(), "/var/lib/memif-dispatch/memif-dispatcher.sock")
	if err != nil {
		logger.Errorf("Failed to dial into Dispatcher with error: %+v", err)
		os.Exit(1)
	}
	client := dispatcher.NewDispatcherClient(clientConn)

	connect := dispatcher.ConnectMsg{
		SrcId: &dispatcher.ID{
			PodName:      "pod1",
			PodNamespace: "pod1-namespace",
		},
		DstId: &dispatcher.ID{
			PodName:      "pod2",
			PodNamespace: "pod2-namespace",
		},
	}
	ready := false
	var sock *dispatcher.SocketMsg
	for !ready {
		sock, err = client.Connect(context.Background(), &connect)
		if err != nil {
			logger.Warnf("Connect call failed with error: %+v retrying in 120 seconds", err)
			time.Sleep(time.Second * 120)
		} else {
			ready = true
		}
	}
	logger.Infof("Successfully get socket: %+v", sock)
	stopCh := signals.SetupSignalHandler()
	<-stopCh
}

func dial(ctx context.Context, unixSocketPath string) (*grpc.ClientConn, error) {
	c, err := grpc.DialContext(ctx, unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	return c, err
}
