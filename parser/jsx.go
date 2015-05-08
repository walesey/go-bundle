package parser

import (
	"bytes"
	"github.com/mamaar/risotto/ast"
	"github.com/mamaar/risotto/file"
	"github.com/mamaar/risotto/token"
)

func (self *_parser) parseJSX() (ast.Expression, error) {
	switch self.token {
	case token.LESS:
		return self.parseJSXElement()
	case token.LEFT_BRACE:
		return self.parseJSXExpression()
	case token.SEMICOLON:
		self.next()
		return self.parsePrimaryExpression()
	}
	return self.parseJSXText()
}

func (self *_parser) parseJSXExpression() (*ast.JSXExpression, error) {

	pos, err := self.expect(token.LEFT_BRACE)
	if err != nil {
		return nil, err
	}
	identifier, err := self.parseExpression()
	if err != nil {
		return nil, err
	}

	variable := &ast.JSXExpression{
		Pos:        pos,
		Identifier: identifier,
	}

	if _, err := self.expect(token.RIGHT_BRACE); err != nil {
		return nil, err
	}

	return variable, nil
}

func (self *_parser) parseJSXText() (*ast.JSXText, error) {
	text := &ast.JSXText{
		Pos: self.idx,
	}
	buf := &bytes.Buffer{}

	for self.token != token.EOF && self.token != token.LEFT_BRACE && self.token != token.LESS {
		buf.WriteString(self.literal)
		if self.literal == "" {
			buf.WriteString(self.token.String())
		}
		self.rawNext()
	}
	text.Literal = buf.String()
	return text, nil
}

func (self *_parser) parseJSXBlock() (*ast.JSXBlock, error) {
	jsx := &ast.JSXBlock{}

	opening, err := self.parseOpeningElement()
	if err != nil {
		return nil, err
	}

	jsx.OpeningElement = opening
	if jsx.OpeningElement.SelfClosing {
		return jsx, nil
	}

	for {
		j, err := self.parseJSX()
		if err != nil {
			return nil, err
		}
		if elem, ok := j.(*ast.JSXElement); ok && !elem.IsOpening {
			jsx.ClosingElement = elem
			break
		}
		jsx.Body = append(jsx.Body, j)
		if self.token == token.EOF {
			break
		}
	}
	return jsx, nil
}

func (self *_parser) parseJSXElement() (ast.Expression, error) {
	self.expect(token.LESS)
	if self.token == token.SLASH {
		return self.parseClosingElement()
	}
	return self.parseJSXBlock()
}

func (self *_parser) parseClosingElement() (*ast.JSXElement, error) {
	var leftTag file.Idx
	var name *ast.Identifier
	var rightTag file.Idx
	var err error

	leftTag, err = self.expect(token.SLASH)
	if err != nil {
		return nil, err
	}
	if self.token == token.IDENTIFIER {
		name, err = self.parseIdentifier()
	}

	rightTag, err = self.expect(token.GREATER)
	if err != nil {
		return nil, err
	}
	return &ast.JSXElement{
		IsOpening: false,
		LeftTag:   leftTag,
		RightTag:  rightTag,
		Name:      name,
	}, nil
}

func (self *_parser) parseOpeningElement() (*ast.JSXElement, error) {
	open := &ast.JSXElement{IsOpening: true}
	open.LeftTag = self.idx

	if self.token == token.IDENTIFIER {
		name, err := self.parseIdentifier()
		if err != nil {
			return nil, err
		}
		open.Name = name
	}
	for self.token == token.IDENTIFIER {
		prop, err := self.parseJSXProperty()
		if err != nil {
			return nil, err
		}
		open.PropertyList = append(open.PropertyList, prop)
	}

	if self.token == token.SLASH {
		self.next()
		rightTag, err := self.expect(token.GREATER)
		if err != nil {
			return nil, err
		}
		open.RightTag = rightTag
		open.SelfClosing = true
		return open, nil
	}
	rightTag, err := self.expect(token.GREATER)
	if err != nil {
		return nil, err
	}
	open.RightTag = rightTag
	return open, nil
}

func (self *_parser) parseJSXProperty() (ast.Property, error) {
	p := ast.Property{}

	id, err := self.parseIdentifier()
	if err != nil {
		return ast.Property{}, err
	}
	p.Key = id.Name
	if _, err := self.expect(token.ASSIGN); err != nil {
		return ast.Property{}, err
	}
	val, err := self.parseJSXValue()
	if err != nil {
		return ast.Property{}, err
	}
	p.Value = val
	return p, nil
}

func (self *_parser) parseJSXValue() (ast.Expression, error) {
	if self.token == token.STRING {
		t := &ast.JSXText{
			Pos:     self.idx,
			Literal: self.literal[1 : len(self.literal)-1],
		}
		self.next()
		return t, nil
	}

	return self.parseJSXExpression()
}
