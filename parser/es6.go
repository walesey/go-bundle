package parser

import (
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
			Argument: self.parseExpression(),
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
