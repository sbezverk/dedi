package utils

import (
	"fmt"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/sbezverk/memif2memif/pkg/types"
)

// AllocateSocket will allocate a socket and return FD
func AllocateSocket(id types.ID) (int, error) {
	uid := uuid.New().String()
	file := path.Join("/var/lib/memif-dispatch/", id.Name+"-"+id.Namespace+"-"+uid[len(uid)-8:])
	fd, err := os.Create(file)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate file descriptor with error: %+v", err)
	}
	return int(fd.Fd()), nil
}
