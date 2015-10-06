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
	s := &section{name: name}
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

// Render the entire configuration of this section and return it as a string list
func (s *Section) render() []string {
	result := []string{s.name}
	for _, o := range s.options {
		line := "    " + o
		result = append(result, line)
	}
	return result
}
