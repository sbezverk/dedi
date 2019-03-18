# DeDi *Descriptors Dispatcher*

WIP

The goal of a **Descriptor Dispatcher (DeDi)** application is to provide a client with a descritor *(file descriptor, socket descriptor or a device)* of a service it wants to connect. 

**DeDi** acts as a middle man between a "server" application, which is offering its service by means of publishing a descriptor and a client, which is requesting it from DeDi in a *Connect* to a service message. "Server" application uses *Listen* message to inform DeDi about descriptors and corresponding services' ids. DeDi stores *services* only until the time a client claims it. Once the client receives the descriptor of the service it needs, DeDi removes this descriptor from the list and let the client and the server to communicate over the descriptor directly. 

DeDi can be integrated with kubernetes by specifying a command line flag **--register=true**, DeDi uses Device Plugin API (DPAPI) to advertise learned services to kubernetes. This integration allows a client application which is running in a pod to request a specific service in its *SPEC* and kubernetes scheduler will land the client pod on a compute node where the requested service is available.

Client and Server applications communicate with DeDi by using the library functions. (*Still TODO*)
