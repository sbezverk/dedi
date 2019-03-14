package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/sbezverk/memif2memif/pkg/apis/dispatcher"
	"github.com/sbezverk/memif2memif/pkg/tools"

	"go.uber.org/zap"
)

const (
	dispatcherSocket = "unix:///var/lib/dispatch/dispatcher.sock"
)

var (
	logger      *zap.SugaredLogger
	dialTimeout = 30 * time.Second
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
	logger.Infof("Starting Listen...")

	t, err := ioutil.TempFile("/tmp/", "test-*")
	if err != nil {
		fmt.Printf("Failed to create a temp file with error: %+v\n", err)
		os.Exit(1)
	}
	defer t.Close()
	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	clientConn, err := dial(ctx, dispatcherSocket)
	if err != nil {
		logger.Errorf("Failed to dial into Dispatcher with error: %+v", err)
		os.Exit(1)
	}
	client := dispatcher.NewDispatcherClient(clientConn)

	listenMsg := dispatcher.ListenMsg{
		PodUuid: "pod2",
		SvcUuid: "service-2",
	}

	sock, err := client.Listen(context.Background(), &listenMsg)
	if err != nil {
		logger.Errorf("Failed to receive socket message from Dispatcher with error: %+v", err)
		os.Exit(1)
	}

	var fds []int
	fds = append(fds, int(t.Fd()))
	rights := syscall.UnixRights(fds...)
	fmt.Printf("File descriptors: %+v\n", fds)
	fmt.Printf("Control messages: %+v\n", rights)

	if err := tools.CheckSocketReadiness(sock.Socket); err != nil {
		fmt.Printf("Failed to wait for the  socket %s to become ready with error: %+v\n", sock.Socket, err)
		os.Exit(2)
	}
	uc, err := net.DialUnix("unixgram", nil, &net.UnixAddr{
		Name: sock.Socket,
		Net:  "unixgram",
	})
	if err != nil {
		fmt.Printf("Failed to Dial socket %s with error: %+v\n", sock.Socket, err)
		os.Exit(2)
	}
	f, _ := uc.File()
	fd := f.Fd()
	if err := syscall.Sendmsg(int(fd), nil, rights, nil, 0); err != nil {
		fmt.Printf("Failed to SendMsg with error: %+v\n", err)
		os.Exit(3)
	}
	fmt.Printf("All good!\n")

	stopCh := make(chan struct{})
	<-stopCh
}

func dial(ctx context.Context, unixSocketPath string) (*grpc.ClientConn, error) {
	c, err := grpc.DialContext(ctx, unixSocketPath, grpc.WithInsecure(), grpc.WithBlock())
	return c, err
}
