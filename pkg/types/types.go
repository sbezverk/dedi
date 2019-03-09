package types

// ID is used to identify pod in the map of known pods
type ID struct {
	Name      string
	Namespace string
}

// Socket identifies a client's socket
type Socket int

// State describe the state of a socket
type State int

const (
	// Unknown state of a socket
	Unknown State = iota
	// Allocated state of a socket
	Allocated
	// Failed state of a socket
	Failed
	// Connected state of a socket
	Connected
)

// Clients is a map of clients IDs, each client can have a map of Sockets and each socket has a state
type Clients struct {
	Client map[ID]map[Socket]State
}
