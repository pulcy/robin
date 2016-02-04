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

package haproxy

import (
	"strings"
)

// Wrapper for a haproxy.conf file
type Config struct {
	sections []*Section
}

type Section struct {
	name    string
	options []string
}

// Create a new configuration
func NewConfig() *Config {
	c := &Config{}
	c.Section("global")
	c.Section("defaults")
	return c
}

// Section returns a name with given section, adding it if needed
func (c *Config) Section(name string) *Section {
	for _, s := range c.sections {
		if s.name == name {
			return s
		}
	}
	s := &Section{name: name}
	c.sections = append(c.sections, s)
	return s
}

// Render the entire configuration and return it as a string
func (c *Config) Render() string {
	lines := []string{}
	for _, s := range c.sections {
		lines = append(lines, s.render()...)
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// Add appends the given options to this section
func (s *Section) Add(options ...string) {
	s.options = append(s.options, options...)
}

// Render the entire configuration of this section and return it as a string list
func (s *Section) render() []string {
	result := []string{s.name}
	for _, o := range s.options {
		line := "    " + o
		result = append(result, line)
	}
	return result
}
