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

package utils

import (
	"gopkg.in/macaron.v1"
)

// DefaultJSON creates a handler that sets the request Content-Type to `application/json` if it was not set before
// and the method is either POST or PUT.
func DefaultJSON() macaron.Handler {
	return func(ctx *macaron.Context) {
		switch ctx.Req.Method {
		case "POST", "PUT":
			hdr := ctx.Req.Header
			if hdr.Get("Content-Type") == "" {
				hdr.Set("Content-Type", "application/json")
			}
		}
	}
}
