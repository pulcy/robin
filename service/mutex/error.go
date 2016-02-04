// Copyright (c) 2016 Pulcy.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mutex

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
