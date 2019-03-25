package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/sbezverk/dedi/pkg/apis/dispatcher"
	"github.com/sbezverk/dedi/pkg/server"
	"github.com/sbezverk/dedi/pkg/tools"

	"go.uber.org/zap"
)

const (
	dispatcherSocket = "/var/lib/dispatch/dispatcher.sock"
)

var (
	logger      *zap.SugaredLogger
	dialTimeout = 30 * time.Second
	svcID       = flag.String("svc-id", "service-1", "Service ID to request from Dispatcher")
	podID       = flag.String("pod-id", "pod-1", "Identity to use for a request")
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
	clientConn, err := tools.Dial(ctx, dispatcherSocket)
	if err != nil {
		logger.Errorf("Failed to dial into Dispatcher with error: %+v", err)
		os.Exit(1)
	}

	client := dispatcher.NewDispatcherClient(clientConn)
	connectMsg := dispatcher.ConnectMsg{
		SvcUuid: *svcID,
	}
	connectMsg.PodUuid = os.Getenv("POD_NAME")
	if connectMsg.PodUuid == "" {
		connectMsg.PodUuid = *podID
	}
	// Informing dispatcher about the service the client wants to connect to.
	// In response, client gets a stream, stream is used for 2 purposes, 1 - recieve from
	// dispatcher the location of the socket to receive the service's descriptor, 2 - for keepalive purposes
	// if Dispatcher detects failure, it will assume that the client is no longer alive and return used connection
	// into the pool of connections.
	stream, err := client.Connect(context.Background(), &connectMsg)
	if err != nil {
		logger.Errorf("Failed to receive socket message from Dispatcher with error: %+v", err)
		os.Exit(1)
	}

	// Loop to wait for the message from Dispatcher carrying the location of a socket.
	var sock *dispatcher.ReplyMsg
	for {
		sock, err = stream.Recv()
		if err != nil {
			logger.Errorf("Failed to receive socket message from Dispatcher with error: %+v", err)
			os.Exit(1)
		}
		logger.Infof("Received message: %+v", sock)
		if err == nil && sock.Error == dispatcher.ERR_NO_ERROR {
			break
		}
	}
	// Once the location of the socket is recevied, client starts listening
	// on this socket to receive Service Descriptor.
	fd, err := server.OpenSocket(sock.Socket)
	if err != nil {
		fmt.Printf("Failed to Open socket %s with error: %+v\n", sock.Socket, err)
		os.Exit(1)
	}
	// Support only 1 message per packet
	buf := make([]byte, syscall.CmsgSpace(4))

	n, oobn, rf, ss, err := syscall.Recvmsg(fd, nil, buf, 0)
	if err != nil {
		fmt.Printf("Failed to Receive massage on socket %s with error: %+v\n", sock.Socket, err)
		os.Exit(1)
	}
	fmt.Printf("n: %d oobn: %d rf: %0x source socket: %+v\n", n, oobn, rf, ss)
	fmt.Printf("Received buffer: %+v\n", buf)
	var msgs []syscall.SocketControlMessage
	msgs, err = syscall.ParseSocketControlMessage(buf)
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
	uid := uuid.New().String()
	fileName := connectMsg.SvcUuid + "-" + uid[len(uid)-8:]

	file := os.NewFile(uintptr(allfds[0]), path.Join("/tmp", fileName))
	if file == nil {
		fmt.Printf("Failed to convert fd into a file.\n")
		os.Exit(4)
	}
	defer file.Close()
	fmt.Printf("Created file object with name: %s \n", file.Name())
	fi, err := file.Stat()
	if err != nil {
		fmt.Printf("Failed to Stat service file: %s with error: %+v\n", file.Name(), err)
		os.Exit(4)
	}
	fmt.Printf("File Stat returnd: %+v\n", fi)
	file.Seek(0, 0)
	b, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Printf("Failed to read service file: %s with error: %+v\n", file.Name(), err)
		os.Exit(4)
	}
	fmt.Printf("Read: %d bytes from file: %s buffer: %s", len(b), file.Name(), string(b))
	stopCh := make(chan struct{})
	<-stopCh
}
