package locks

import (
	"github.com/juju/errgo"
)

var (
	InvalidArgumentError = errgo.New("invalid argument")
	AlreadyLockedError   = errgo.New("already locked")
	NotLockedError       = errgo.New("not locked yet")
	NotOwnerError        = errgo.New("not owner")
	AlreadyUsedError     = errgo.New("cannot claim lock twice")
	maskAny              = errgo.MaskFunc(errgo.Any)
)

func IsInvalidArgument(err error) bool {
	return errgo.Cause(err) == InvalidArgumentError
}

func IsAlreadyLocked(err error) bool {
	return errgo.Cause(err) == AlreadyLockedError
}

func IsAlreadyUsed(err error) bool {
	return errgo.Cause(err) == AlreadyUsedError
}

func IsNotLocked(err error) bool {
	return errgo.Cause(err) == NotLockedError
}

func IsNotOwner(err error) bool {
	return errgo.Cause(err) == NotOwnerError
}
