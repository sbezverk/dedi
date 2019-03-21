package controller

import (
	"github.com/sbezverk/dedi/pkg/registration"
	"go.uber.org/zap"
)

type resource struct {
	stopCh   chan struct{}
	updateCh chan struct{}
	rm       registration.ResourceManager
}

type resourceController struct {
	logger    *zap.SugaredLogger
	stopCh    chan struct{}
	updateCh  chan string
	resources map[string]resource
}

// ResourceController is interface to access resourceController resources
type ResourceController interface {
	Run()
	Shutdown()
}

// NewResourceController creates an instance of a new resourceCOntroller and returns its interface
func NewResourceController(logger *zap.SugaredLogger, updateCh chan string) ResourceController {
	return &resourceController{
		logger:   logger,
		stopCh:   make(chan struct{}),
		updateCh: updateCh,
	}
}

func (rc *resourceController) Run() {
	for {
		select {
		case msg := <-rc.updateCh:
			rc.logger.Infof("Received update message from Dispatcher: %s", msg)
		case <-rc.stopCh:
			rc.logger.Infof("Received Shutdown message, shutting down...")
			rc.Shutdown()
		}
	}
}

func (rc *resourceController) Shutdown() {

}
