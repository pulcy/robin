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

// Ping creates a handler that returns an OK response.
func Ping() macaron.Handler {
	return func(ctx *macaron.Context) {
		if ctx.Req.Method == "HEAD" {
			ctx.Header().Set("Content-Length", "0")
			ctx.PlainText(200, []byte(""))
			return
		}
		status := struct {
			Status string `json:"status"`
		}{
			Status: "OK",
		}
		ctx.JSON(200, status)
	}
}
