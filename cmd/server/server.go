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

	"github.com/sbezverk/dedi/pkg/apis/dispatcher"
	"github.com/sbezverk/dedi/pkg/tools"

	"go.uber.org/zap"
)

const (
	dispatcherSocket = "/var/lib/dispatch/dispatcher.sock"
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
	// Writing some info into the file to read on the other side
	t.WriteString(time.Now().String() + "\n")
	if err := os.Chmod(t.Name(), 0644); err != nil {
		fmt.Printf("Failed to set permissions on the created temp file with error: %+v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()
	clientConn, err := tools.Dial(ctx, dispatcherSocket)
	if err != nil {
		logger.Errorf("Failed to dial into Dispatcher with error: %+v", err)
		os.Exit(1)
	}
	client := dispatcher.NewDispatcherClient(clientConn)

	listenMsg := dispatcher.ListenMsg{
		PodUuid:        "pod2",
		SvcUuid:        "service-2",
		MaxConnections: int32(1000),
	}

	// Getting Unix Domain Socket for SendMsg via Listen gRPC call.
	stream, err := client.Listen(context.Background(), &listenMsg)
	if err != nil {
		logger.Errorf("Failed to receive socket message from Dispatcher with error: %+v", err)
		os.Exit(1)
	}

	sock, err := stream.Recv()
	if err != nil {
		logger.Errorf("Failed to receive socket message from Dispatcher with error: %+v", err)
		os.Exit(1)
	}
	go connectionHandler(stream)

	rights := syscall.UnixRights(int(t.Fd()))

	fmt.Printf("File descriptor: %d\n", int(t.Fd()))
	fi, err := t.Stat()
	if err != nil {
		fmt.Printf("Failed to Stat service file: %s with error: %+v\n", t.Name(), err)
		os.Exit(4)
	}
	fmt.Printf("File Stat returnd: %+v\n", fi)

	fmt.Printf("Encoded Socket Control Messages: %+v\n", rights)

	if err := tools.CheckSocketReadiness(sock.Socket); err != nil {
		fmt.Printf("Failed to wait for the  socket %s to become ready with error: %+v\n", sock.Socket, err)
		os.Exit(2)
	}
	// Bundle it into a library call
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

func connectionHandler(stream dispatcher.Dispatcher_ListenClient) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return
		}
		fmt.Printf("Received message: %+v\n", *msg)
	}
}
