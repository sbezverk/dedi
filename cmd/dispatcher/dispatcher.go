package main

import (
	"flag"
	"net"
	"os"

	// "golang.org/x/sys/unix"
	"google.golang.org/grpc"
	// "google.golang.org/grpc/status"

	"github.com/knative/pkg/signals"
	"github.com/sbezverk/memif2memif/pkg/apis/dispatcher"
	"github.com/sbezverk/memif2memif/pkg/server"
	"github.com/sbezverk/memif2memif/pkg/tools"

	"go.uber.org/zap"
)

const (
	dispatcherSocket = "/var/lib/dispatch/dispatcher.sock"
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
	flag.Parse()
	// Advertise via DPAPI

	if err := tools.SocketCleanup(dispatcherSocket); err != nil {
		logger.Errorf("Failed to cleaup stale socket with error: %+v", err)
		os.Exit(1)
	}
	// Setting up gRPC server
	listener, err := net.Listen("unix", dispatcherSocket)
	if err != nil {
		logger.Errorf("Failed to setup listener with error: %+v", err)
		os.Exit(1)
	}
	srv := grpc.NewServer([]grpc.ServerOption{}...)

	// Attaching Dispatcher API
	dispatcher.RegisterDispatcherServer(srv, server.NewDispatcher(logger))

	stopCh := signals.SetupSignalHandler()
	logger.Infof("Dispatcher is starting...")
	go func() {
		if err := srv.Serve(listener); err != nil {
			logger.Errorw("Error running gRPC server", zap.Error(err))
		}
	}()
	<-stopCh
	// Can signal to go routines to shutdown gracefully
}
