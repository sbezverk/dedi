package dispatcher

//go:generate protoc -I . dispatcher.proto --go_out=plugins=grpc:. --proto_path=$GOPATH/src
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:deepcopy-gen=package,register
