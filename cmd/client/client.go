package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/sbezverk/memif2memif/pkg/apis/dispatcher"

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
	logger.Infof("Starting Connect...")

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	clientConn, err := grpc.DialContext(ctx, dispatcherSocket, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		logger.Errorf("Failed to dial into Dispatcher with error: %+v", err)
		os.Exit(1)
	}
	client := dispatcher.NewDispatcherClient(clientConn)

	connectMsg := dispatcher.ConnectMsg{
		PodUuid: "pod1",
		SvcUuid: "service-2",
	}

	sock, err := client.Connect(context.Background(), &connectMsg)
	if err != nil {
		logger.Errorf("Failed to receive socket message from Dispatcher with error: %+v", err)
		os.Exit(1)
	}

	uc, err := net.ListenUnixgram("unixgram", &net.UnixAddr{
		Name: sock.Socket,
		Net:  "unixgram",
	})
	if err != nil {
		fmt.Printf("Failed to listen on socket %s with error: %+v\n", sock.Socket, err)
		os.Exit(1)
	}

	f, _ := uc.File()
	fd := f.Fd()
	num := 1

	buf := make([]byte, syscall.CmsgSpace(num*4))

	n, oobn, rf, ss, err := syscall.Recvmsg(int(fd), nil, buf, 0)
	if err != nil {
		panic(err)
	}
	fmt.Printf("n: %d oobn: %d rf: %0x source socket: %+v\n", n, oobn, rf, ss)
	fmt.Printf("Received buffer: %+v\n", buf)
	var msgs []syscall.SocketControlMessage
	msgs, err = syscall.ParseSocketControlMessage(buf[:oobn])
	if err != nil {
		fmt.Printf("Failed to parse messages with error: %+v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Received messages: %+v\n", msgs)
	var allfds []int
	for i := 0; i < len(msgs) && err == nil; i++ {
		var msgfds []int
		msgfds, err = syscall.ParseUnixRights(&msgs[i])
		allfds = append(allfds, msgfds...)
	}
	fmt.Printf("Parsed right: %+v\n", allfds)

	stopCh := make(chan struct{})
	<-stopCh
}

func dial(ctx context.Context, unixSocketPath string) (*grpc.ClientConn, error) {
	c, err := grpc.DialContext(ctx, unixSocketPath, grpc.WithInsecure(), grpc.WithBlock())
	return c, err
}
