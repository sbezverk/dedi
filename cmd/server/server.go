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
	svcID       = flag.String("svc-id", "service-1", "Service ID to register with Dispatcher.")
	podID       = flag.String("pod-id", "pod-1", "POD ID to register with Dispatcher.")
	maxConns    = flag.Int("max-connections", 1, "Maximum number of connections for a service")
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
		SvcUuid:        *svcID,
		MaxConnections: int32(*maxConns),
	}
	listenMsg.PodUuid = os.Getenv("POD_NAME")
	if listenMsg.PodUuid == "" {
		listenMsg.PodUuid = *podID
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

	logger.Infof("File descriptor: %d", int(t.Fd()))
	fi, err := t.Stat()
	if err != nil {
		logger.Errorf("Failed to Stat service file: %s with error: %+v", t.Name(), err)
		os.Exit(4)
	}
	logger.Infof("File Stat returnd: %+v", fi)

	logger.Infof("Encoded Socket Control Messages: %+v", rights)

	if err := tools.CheckSocketReadiness(sock.Socket); err != nil {
		logger.Errorf("Failed to wait for the  socket %s to become ready with error: %+v", sock.Socket, err)
		os.Exit(2)
	}
	// Bundle it into a library call
	uc, err := net.DialUnix("unixgram", nil, &net.UnixAddr{
		Name: sock.Socket,
		Net:  "unixgram",
	})
	if err != nil {
		logger.Errorf("Failed to Dial socket %s with error: %+v", sock.Socket, err)
		os.Exit(2)
	}
	f, _ := uc.File()
	fd := f.Fd()
	if err := syscall.Sendmsg(int(fd), nil, rights, nil, 0); err != nil {
		logger.Errorf("Failed to SendMsg with error: %+v", err)
		os.Exit(3)
	}
	logger.Infof("All good!")

	stopCh := make(chan struct{})
	<-stopCh
}

func connectionHandler(stream dispatcher.Dispatcher_ListenClient) {
	for {
		_, err := stream.Recv()
		if err != nil {
			return
		}
	}
}
