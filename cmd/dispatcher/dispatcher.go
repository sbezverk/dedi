package main

import (
	"flag"
	"os"

	"github.com/knative/pkg/signals"
	"github.com/sbezverk/memif2memif/pkg/controller"
	"github.com/sbezverk/memif2memif/pkg/server"

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
	updateCh := make(chan struct{})

	dispatch, err := server.NewDispatcher(dispatcherSocket, logger, updateCh)
	if err != nil {
		logger.Errorf("Failed to instantiate Dispatcher with error: %+v", err)
	}
	logger.Infof("Dispatcher is starting...")
	go func() {
		if err := dispatch.Run(); err != nil {
			logger.Errorw("Error running gRPC server", zap.Error(err))
		}
	}()

	// Preparing dpapi controller
	controller := controller.NewResourceController(logger, updateCh)
	stopCh := signals.SetupSignalHandler()
	<-stopCh
	// Can signal to go routines to shutdown gracefully
	dispatch.Shutdown()
	controller.Shutdown()
	os.Exit(0)
}
