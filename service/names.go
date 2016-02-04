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

package service

import (
	"fmt"
)

type nameGenerator struct {
	prefix string
	last   int
}

func NewNameGenerator(prefix string) *nameGenerator {
	return &nameGenerator{
		prefix: prefix,
		last:   0,
	}
}

func (ng *nameGenerator) Next() string {
	ng.last++
	return fmt.Sprintf("%s%d", ng.prefix, ng.last)
}
