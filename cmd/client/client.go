package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/google/uuid"
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
	defer func() {
		// Closing Unix Domain Socket used to receive Descriptor
		syscall.Close(int(fd))
		// Removing socket file
		os.Remove(sock.Socket)
	}()
	buf := make([]byte, syscall.CmsgSpace(num*4))

	n, oobn, rf, ss, err := syscall.Recvmsg(int(fd), nil, buf, 0)
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
