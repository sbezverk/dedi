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
	flag.Parse()
	// Advertise via DPAPI

	// Setting up gRPC server
	listener, err := net.Listen("unix", "/var/lib/memif-dispatch/memif-dispatcher.sock")
	if err != nil {
		logger.Errorf("Failed to setup listener with error", err)
	}
	srv := grpc.NewServer([]grpc.ServerOption{}...)

	// Attaching Dispatcher API
	dispatcher.RegisterDispatcherServer(srv, server.NewMemifDispatcher(logger))

	stopCh := signals.SetupSignalHandler()
	logger.Infof("WIP Dispatcher is starting...")
	go func() {
		if err := srv.Serve(listener); err != nil {
			logger.Errorw("Error running gRPC server", zap.Error(err))
		}
	}()
	<-stopCh
	// Can signal to go routines to shutdown gracefully
}
