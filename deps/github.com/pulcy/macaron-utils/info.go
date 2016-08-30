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
	"fmt"

	"gopkg.in/macaron.v1"
)

// ServerInfo creates a handler that sets returns a text formatted version info.
func ServerInfo(projectName, projectVersion, projectBuild string) macaron.Handler {
	return func(ctx *macaron.Context) string {
		if ctx.Req.Method == "HEAD" {
			ctx.Resp.Header().Set("Content-Length", "0")
			return ""
		}
		return fmt.Sprintf("%s, version %s build %s", projectName, projectVersion, projectBuild)
	}
}
