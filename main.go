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

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/op/go-logging"
	"github.com/spf13/cobra"
)

var (
	projectVersion = "dev"
	projectBuild   = "dev"
)

var (
	cmdMain = &cobra.Command{
		Use:   "load-balancer",
		Short: "Distribute incoming requests onto configurable backends",
		Long:  "Distribute incoming requests onto configurable backends",
		Run:   UsageFunc,
	}
	log = logging.MustGetLogger(cmdMain.Use)
)

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:-5s}] %{message}"))
}

func main() {
	cmdMain.Execute()
}

func UsageFunc(cmd *cobra.Command, args []string) {
	cmd.Usage()
}

func Exitf(format string, args ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format = format + "\n"
	}
	fmt.Printf(format, args...)
	os.Exit(1)
}

func def(envKey, defaultValue string) string {
	s := os.Getenv(envKey)
	if s == "" {
		s = defaultValue
	}
	return s
}
