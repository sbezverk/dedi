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
	// Listen state of a socket
	Listen
)

// Client can have a map of Sockets and each socket has a state for Listen and/or Connect
type Client struct {
	// MaxConnections sets by the listener to indicate the number of connections it supports
	MaxConnections int32
	Listen         map[Socket]State
	Connect        map[Socket]State
}

// Clients is a map of clients IDs each client can have a map of Sockets and each socket has a state
type Clients struct {
	Clients map[ID]Client
}
