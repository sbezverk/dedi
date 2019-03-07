# memif2memif

WIP

It is an attempt to build a memif manager application, it runs as a daemonset on each compute node, it advertises itself to kubernetes via dpapi. The client, which is an application requiring connectivity and the server, which is application offering its service via memif interface, communicates with the dispatcher over a Unix domain socket and exchanges gRPC messages to build memif to memif connection. The dispatcher creates file descriptors and distribute them to both applications, the applications then will do standard memif negotiation. 