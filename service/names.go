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
