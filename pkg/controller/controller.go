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
			rc.handleUpdate(msg)
		case <-rc.stopCh:
			rc.logger.Infof("Received Shutdown message, shutting down...")
			rc.Shutdown()
		}
	}
}

func (rc *resourceController) Shutdown() {

}

func (rc *resourceController) handleUpdate(msg types.UpdateOp) {
	switch msg.Op {
	case types.Add:
		rc.logger.Infof("resource controller: Adding service: %s with maximum connections: %d", msg.ServiceID, msg.AvailableConnections)
		go rc.addService(msg)
	case types.Delete:
		rc.logger.Infof("resource controller: Deleting service: %s", msg.ServiceID)
		go rc.deleteService(msg)
	case types.Update:
		rc.logger.Infof("resource controller: Updating service: %s with number of available connections: %d", msg.ServiceID, msg.AvailableConnections)
		go rc.updateService(msg)
	default:
		rc.logger.Warnf("resource controller: Unknown operation in message: %+v", msg)
	}
}

func (rc *resourceController) addService(msg types.UpdateOp) {
	// Check if the service with this ID already exists, do nothing if it does
	if _, ok := rc.resources[msg.ServiceID]; ok {
		return
	}
}

func (rc *resourceController) deleteService(msg types.UpdateOp) {
	// Check if the service with this ID does not exist, do nothing if it does not
	if _, ok := rc.resources[msg.ServiceID]; !ok {
		return
	}
}

func (rc *resourceController) updateService(msg types.UpdateOp) {
	// Check if the service with this ID does not exist, do nothing if it does not
	if _, ok := rc.resources[msg.ServiceID]; !ok {
		return
	}
}
