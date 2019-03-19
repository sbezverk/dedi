package main

import (
	"flag"
	"os"

	"github.com/knative/pkg/signals"
	"github.com/sbezverk/dedi/pkg/controller"
	"github.com/sbezverk/dedi/pkg/server"

	"go.uber.org/zap"
)

const (
	dispatcherSocket = "/var/lib/dispatch/dispatcher.sock"
)

var (
	logger   *zap.SugaredLogger
	register = flag.Bool("register", false, "set to true if registration with kubelet is required, default set to flase")
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
		os.Exit(1)
	}
	logger.Infof("Dispatcher is starting...")
	go func() {
		if err := dispatch.Run(); err != nil {
			logger.Errorw("Error running gRPC server", zap.Error(err))
			os.Exit(2)
		}
	}()

	// Only run resource controller if registration to kubelet is required
	var rc controller.ResourceController
	logger.Infof("Registration with kubelet flag is set to: %t", *register)
	if *register {
		// Preparing dpapi controller
		rc, err = controller.NewResourceController(logger, updateCh)
		if err != nil {
			logger.Errorf("Failed to instantiate Resource Controller with error: %+v", err)
			os.Exit(3)
		}
		logger.Infof("Resource Controller is starting...")
		if err := rc.Run(); err != nil {
			logger.Errorw("Error running Resource Controller gRPC server", zap.Error(err))
			os.Exit(4)
		}
	}
	stopCh := signals.SetupSignalHandler()
	<-stopCh
	// Can signal to go routines to shutdown gracefully
	dispatch.Shutdown()
	if *register {
		rc.Shutdown()
	}
	os.Exit(0)
}
