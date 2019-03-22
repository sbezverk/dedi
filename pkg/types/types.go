package types

// Operation defines Operation type
type Operation int

const (
	// Add defines add new resource operation
	Add Operation = iota
	// Delete defines delete of the existing resource operation
	Delete
	// Update defines update of currenrly available conenction of the existing resource
	Update
)

// UpdateOp defines a struct which is used to carry ovwer update channel changes to resources
type UpdateOp struct {
	Op                   Operation
	ServiceID            string
	MaxConnections       int32
	AvailableConnections int32
}
