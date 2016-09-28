package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/walesey/go-bundle/ast"
)

var validJSX = []string{
	"<div />",
	"<div param=\"value\"></div>",
	"<div><div /></div>",
	"<div prop={name} />",
}

func p(jsx string) (*ast.Program, error) {
	p := _newParser("", jsx, 1)
	return p.parse()
}

func TestJSX(t *testing.T) {
	for _, jsx := range validJSX {
		_, err := p(jsx)

		assert.NoError(t, err)

	}
}
