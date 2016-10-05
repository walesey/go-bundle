package parser

import (
	"fmt"
	"regexp"

	"github.com/walesey/go-bundle/ast"
	"github.com/walesey/go-bundle/token"
)

func (self *_parser) parseArrowFunction(params *ast.ParameterList) *ast.FunctionLiteral {
	node := &ast.FunctionLiteral{
		Function: self.expect(token.ARROW),
	}

	if self.mode&StoreComments != 0 {
		self.comments.Unset()
	}
	node.ParameterList = params
	if self.token == token.LEFT_BRACE {
		self.parseFunctionBlock(node)
	} else {
		leftBrace := self.idx
		stmt := &ast.ReturnStatement{
			Return:   leftBrace,
			Argument: self.parseAssignmentExpression(),
		}
		node.Body = &ast.BlockStatement{
			LeftBrace:  leftBrace,
			List:       []ast.Statement{stmt},
			RightBrace: self.idx,
		}
	}
	node.Source = self.slice(node.Idx0(), node.Idx1())

	return node
}

func (self *_parser) parseImportStatement() ast.Statement {
	node := &ast.ImportStatement{
		Import: self.expect(token.IMPORT),
	}

	if self.token == token.IDENTIFIER {
		node.Default = self.parseIdentifier()
		if self.token == token.COMMA {
			self.next()
		}
	}

	if self.token == token.LEFT_BRACE {
		self.expect(token.LEFT_BRACE)
		node.List = self.parseIdentifierList()
		self.expect(token.RIGHT_BRACE)
	}

	if self.token != token.IDENTIFIER {
		self.errorUnexpectedToken(self.token)
	}

	from := self.parseIdentifier()
	if from.Name != "from" {
		self.error(self.idx, "Expected import Statement to be followed by 'from'.")
	}

	literal := self.literal
	idx := self.idx

	self.expect(token.STRING)
	value, err := parseStringLiteral(literal[1 : len(literal)-1])
	if err != nil {
		self.error(idx, err.Error())
	}

	node.Path = &ast.StringLiteral{
		Idx:     idx,
		Literal: literal,
		Value:   value,
	}
	return node
}

func (self *_parser) parseExportStatement() ast.Statement {
	export := self.expect(token.EXPORT)

	if self.token == token.DEFAULT {
		self.expect(token.DEFAULT)
		return &ast.ExportDefaultStatement{
			Export:   export,
			Argument: self.parseExpression(),
		}
	}

	if self.token == token.FUNCTION {
		return &ast.ExportStatement{
			Export: export,
			Statement: &ast.FunctionStatement{
				Function: self.parseFunction(false),
			},
		}
	}

	if self.token != token.VAR && self.token != token.CONST && self.token != token.LET {
		self.errorUnexpectedToken(self.token)
	}

	return &ast.ExportStatement{
		Export:    export,
		Statement: self.parseVariableStatement(),
	}
}

func (self *_parser) parseDestructureVariableStatement() []ast.Expression {
	self.expect(token.LEFT_BRACE)
	identifierList := self.parseIdentifierList()
	self.expect(token.RIGHT_BRACE)
	self.expect(token.ASSIGN)
	initializer := self.parseAssignmentExpression()

	result := make([]ast.Expression, len(identifierList))
	for i, identifier := range identifierList {
		result[i] = &ast.VariableExpression{
			Name: identifier.Name,
			Idx:  identifier.Idx,
			Initializer: &ast.DotExpression{
				Identifier: &ast.Identifier{
					Idx:  self.idx,
					Name: identifier.Name,
				},
				Left: initializer,
			},
		}
	}

	return result
}

func (self *_parser) parseDynamicString() ast.Expression {
	idx := self.expect(token.TEMPLATE)
	list := []ast.Expression{}

	for {
		literal := self.literal
		strIdx := self.idx
		value, err := self.scanTemplateString(self.chrOffset)
		value = fmt.Sprint(literal, value)
		value = regexp.MustCompile("(\\r|\\n)").ReplaceAllString(value, "\\$1")
		literal = fmt.Sprintf("'%v'", value)

		// case '\r':
		// 	if self.chr == '\n' {
		// 		self.chr = '\\n'
		// 	}
		// case '\u2028', '\u2029', '\n':

		if err != nil {
			self.error(self.idx, "error scanning template string: ", err)
		}
		list = append(list, &ast.StringLiteral{
			Idx:     strIdx,
			Literal: literal,
			Value:   value,
		})

		chr := self.chr
		self.rawNext()
		if chr == '`' || chr < 0 {
			break
		}
		if chr != '$' || self.chr != '{' {
			self.error(self.idx, "expected ${...} in template string")
		}
		self.rawNext()
		self.rawNext()
		list = append(list, self.parsePrimaryExpression())
		if self.token != token.RIGHT_BRACE {
			self.errorUnexpectedToken(self.token)
		}
	}

	self.expect(token.TEMPLATE)
	return &ast.DynamicStringExpression{
		Idx:  idx,
		List: list,
	}
}
