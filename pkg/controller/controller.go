package controller

import (
	"github.com/sbezverk/dedi/pkg/registration"
	"github.com/sbezverk/dedi/pkg/types"
	"go.uber.org/zap"
)

type resource struct {
	stopCh chan struct{}
	// connectionsUpdateCh is used to communicate any changes in number of available connections
	connectionsUpdateCh chan int
	rm                  registration.ResourceManager
}

type resourceController struct {
	logger    *zap.SugaredLogger
	stopCh    chan struct{}
	updateCh  chan types.UpdateOp
	resources map[string]resource
}

// ResourceController is interface to access resourceController resources
type ResourceController interface {
	Run()
	Shutdown()
}

// NewResourceController creates an instance of a new resourceCOntroller and returns its interface
func NewResourceController(logger *zap.SugaredLogger, updateCh chan types.UpdateOp) ResourceController {
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
