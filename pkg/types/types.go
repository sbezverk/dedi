package types

import (
	"sync"
)

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
	lockListen     sync.Mutex
	Listen         map[Socket]State
	lockConnect    sync.Mutex
	Connect        map[Socket]State
}

// LockListen locks for Listen Socket map for update
func (c *Client) LockListen() {
	c.lockListen.Lock()
}

// UnlockListen unlocks for Listen Socket map
func (c *Client) UnlockListen() {
	c.lockListen.Unlock()
}

// LockConnect locks Connect Socket map for update
func (c *Client) LockConnect() {
	c.lockConnect.Lock()
}

// UnlockConnect unlocks Connect Socket map
func (c *Client) UnlockConnect() {
	c.lockConnect.Unlock()
}

// Clients is a map of clients IDs each client can have a map of Sockets and each socket has a state
type Clients struct {
	lock    sync.Mutex
	Clients map[ID]*Client
}

// Lock will lock Clients map for exclusive access
func (c *Clients) Lock() {
	c.lock.Lock()
}

// Unlock will unlock Clients map for access
func (c *Clients) Unlock() {
	c.lock.Unlock()
}
