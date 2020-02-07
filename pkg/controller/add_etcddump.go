package controller

import (
	"github.com/gok8s/etcd-operator/pkg/controller/etcddump"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, etcddump.Add)
}
