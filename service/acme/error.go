package acme

import (
	"github.com/juju/errgo"
)

var (
	NotMasterError = errgo.New("not master")
	maskAny        = errgo.MaskFunc(errgo.Any)
)

func IsNotMaster(err error) bool {
	return errgo.Cause(err) == NotMasterError
}
