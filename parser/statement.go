package parser

import (
	"github.com/mamaar/risotto/ast"
	"github.com/mamaar/risotto/token"
)

func (self *_parser) parseBlockStatement() (*ast.BlockStatement, error) {
	leftBrace, err := self.expect(token.LEFT_BRACE)
	if err != nil {
		return nil, err
	}
	list, err := self.parseStatementList()
	if err != nil {
		return nil, err
	}
	rightBrace, err := self.expect(token.RIGHT_BRACE)
	if err != nil {
		return nil, err
	}

	return &ast.BlockStatement{
		LeftBrace:  leftBrace,
		List:       list,
		RightBrace: rightBrace,
	}, nil
}

func (self *_parser) parseEmptyStatement() (ast.Statement, error) {
	idx, err := self.expect(token.SEMICOLON)
	if err != nil {
		return nil, err
	}
	return &ast.EmptyStatement{Semicolon: idx}, nil
}

func (self *_parser) parseStatementList() (list []ast.Statement, err error) {
	for self.token != token.RIGHT_BRACE && self.token != token.EOF {
		var stmt ast.Statement
		stmt, err = self.parseStatement()
		if err != nil {
			return
		}
		list = append(list, stmt)
	}

	return
}

func (self *_parser) parseStatement() (ast.Statement, error) {
	if self.token == token.EOF {
		return nil, self.errorUnexpectedToken(self.token)
	}

	switch self.token {
	case token.SEMICOLON:
		return self.parseEmptyStatement()
	case token.LEFT_BRACE:
		return self.parseBlockStatement()
	case token.IF:
		return self.parseIfStatement()
	case token.DO:
		return self.parseDoWhileStatement()
	case token.WHILE:
		return self.parseWhileStatement()
	case token.FOR:
		return self.parseForOrForInStatement()
	case token.BREAK:
		return self.parseBreakStatement()
	case token.CONTINUE:
		return self.parseContinueStatement()
	case token.DEBUGGER:
		return self.parseDebuggerStatement()
	case token.WITH:
		return self.parseWithStatement()
	case token.VAR:
		return self.parseVariableStatement()
	case token.FUNCTION:
		self.parseFunction(true)
		// FIXME
		return &ast.EmptyStatement{}, nil
	case token.SWITCH:
		return self.parseSwitchStatement()
	case token.RETURN:
		return self.parseReturnStatement()
	case token.THROW:
		return self.parseThrowStatement()
	case token.TRY:
		return self.parseTryStatement()
	}

	expression, err := self.parseExpression()
	if err != nil {
		return nil, err
	}

	if identifier, isIdentifier := expression.(*ast.Identifier); isIdentifier && self.token == token.COLON {
		// LabelledStatement
		colon := self.idx
		self.next() // :
		label := identifier.Name
		for _, value := range self.scope.labels {
			if label == value {
				return nil, self.error(identifier.Idx0(), "Label '%s' already exists", label)
			}
		}
		self.scope.labels = append(self.scope.labels, label) // Push the label
		statement, err := self.parseStatement()
		if err != nil {
			return nil, err
		}
		self.scope.labels = self.scope.labels[:len(self.scope.labels)-1] // Pop the label
		return &ast.LabelledStatement{
			Label:     identifier,
			Colon:     colon,
			Statement: statement,
		}, nil
	}

	self.optionalSemicolon()

	return &ast.ExpressionStatement{
		Expression: expression,
	}, nil
}

func (self *_parser) parseTryStatement() (ast.Statement, error) {
	try, err := self.expect(token.TRY)
	if err != nil {
		return nil, err
	}

	body, err := self.parseBlockStatement()
	if err != nil {
		return nil, err
	}

	node := &ast.TryStatement{
		Try:  try,
		Body: body,
	}

	if self.token == token.CATCH {
		catch := self.idx
		self.next()
		if _, err := self.expect(token.LEFT_PARENTHESIS); err != nil {
			return nil, err
		}
		if self.token != token.IDENTIFIER {
			if _, err := self.expect(token.IDENTIFIER); err != nil {
				return nil, err
			}
		} else {
			identifier, err := self.parseIdentifier()
			if err != nil {
				return nil, err
			}
			if _, err := self.expect(token.RIGHT_PARENTHESIS); err != nil {
				return nil, err
			}
			stmt, err := self.parseBlockStatement()
			if err != nil {
				return nil, err
			}
			node.Catch = &ast.CatchStatement{
				Catch:     catch,
				Parameter: identifier,
				Body:      stmt,
			}
		}
	}

	if self.token == token.FINALLY {
		self.next()
		finally, err := self.parseBlockStatement()
		if err != nil {
			return nil, err
		}
		node.Finally = finally
	}

	if node.Catch == nil && node.Finally == nil {
		return nil, self.error(node.Try, "Missing catch or finally after try")
	}

	return node, nil
}

func (self *_parser) parseFunctionParameterList() (*ast.ParameterList, error) {
	opening, err := self.expect(token.LEFT_PARENTHESIS)
	if err != nil {
		return nil, err
	}
	var list []*ast.Identifier
	for self.token != token.RIGHT_PARENTHESIS && self.token != token.EOF {
		if self.token != token.IDENTIFIER {
			if _, err := self.expect(token.IDENTIFIER); err != nil {
				return nil, err
			}
		} else {
			id, err := self.parseIdentifier()
			if err != nil {
				return nil, err
			}
			list = append(list, id)
		}
		if self.token != token.RIGHT_PARENTHESIS {
			if _, err := self.expect(token.COMMA); err != nil {
				return nil, err
			}
		}
	}
	closing, err := self.expect(token.RIGHT_PARENTHESIS)
	if err != nil {
		return nil, err
	}

	return &ast.ParameterList{
		Opening: opening,
		List:    list,
		Closing: closing,
	}, nil
}

func (self *_parser) parseParameterList() (list []string) {
	for self.token != token.EOF {
		if self.token != token.IDENTIFIER {
			self.expect(token.IDENTIFIER)
		}
		list = append(list, self.literal)
		self.next()
		if self.token != token.EOF {
			self.expect(token.COMMA)
		}
	}
	return
}

func (self *_parser) parseFunction(declaration bool) (*ast.FunctionLiteral, error) {
	idx, err := self.expect(token.FUNCTION)
	if err != nil {
		return nil, err
	}
	node := &ast.FunctionLiteral{
		Function: idx,
	}

	var name *ast.Identifier
	if self.token == token.IDENTIFIER {
		name, err = self.parseIdentifier()
		if err != nil {
			return nil, err
		}
		if declaration {
			self.scope.declare(&ast.FunctionDeclaration{
				Function: node,
			})
		}
	} else if declaration {
		// Use expect error handling
		if _, err := self.expect(token.IDENTIFIER); err != nil {
			return nil, err
		}
	}
	params, err := self.parseFunctionParameterList()
	if err != nil {
		return nil, err
	}

	node.Name = name
	node.ParameterList = params
	if err := self.parseFunctionBlock(node); err != nil {
		return nil, err
	}
	node.Source = self.slice(node.Idx0(), node.Idx1())

	return node, nil
}

func (self *_parser) parseFunctionBlock(node *ast.FunctionLiteral) error {
	{
		self.openScope()
		inFunction := self.scope.inFunction
		self.scope.inFunction = true
		defer func() {
			self.scope.inFunction = inFunction
			self.closeScope()
		}()
		body, err := self.parseBlockStatement()
		if err != nil {
			return err
		}
		node.Body = body
		node.DeclarationList = self.scope.declarationList
	}
	return nil
}

func (self *_parser) parseDebuggerStatement() (ast.Statement, error) {
	idx, err := self.expect(token.DEBUGGER)
	if err != nil {
		return nil, err
	}
	if err := self.semicolon(); err != nil {
		return nil, err
	}

	return &ast.DebuggerStatement{
		Debugger: idx,
	}, nil
}

func (self *_parser) parseReturnStatement() (ast.Statement, error) {
	idx, err := self.expect(token.RETURN)
	if err != nil {
		return nil, err
	}

	if !self.scope.inFunction {
		if err := self.error(idx, "Illegal return statement"); err != nil {
			return nil, err
		}
	}

	node := &ast.ReturnStatement{
		Return: idx,
	}

	if !self.implicitSemicolon && self.token != token.SEMICOLON && self.token != token.RIGHT_BRACE && self.token != token.EOF {
		exp, err := self.parseExpression()
		if err != nil {
			return nil, err
		}
		node.Argument = exp
	}

	if err := self.semicolon(); err != nil {
		return nil, err
	}

	return node, nil
}

func (self *_parser) parseThrowStatement() (ast.Statement, error) {
	idx, err := self.expect(token.THROW)
	if err != nil {
		return nil, err
	}

	if self.implicitSemicolon {
		if self.chr == -1 { // Hackish
			return nil, self.error(idx, "Unexpected end of input")
		}
		return nil, self.error(idx, "Illegal newline after throw")
	}

	exp, err := self.parseExpression()
	if err != nil {
		return nil, err
	}
	if err := self.semicolon(); err != nil {
		return nil, err
	}
	return &ast.ThrowStatement{
		Argument: exp,
	}, nil
}

func (self *_parser) parseSwitchStatement() (ast.Statement, error) {
	if _, err := self.expect(token.SWITCH); err != nil {
		return nil, err
	}
	if _, err := self.expect(token.LEFT_PARENTHESIS); err != nil {
		return nil, err
	}

	discriminant, err := self.parseExpression()
	if err != nil {
		return nil, err
	}

	if _, err := self.expect(token.RIGHT_PARENTHESIS); err != nil {
		return nil, err
	}
	if _, err := self.expect(token.LEFT_BRACE); err != nil {
		return nil, err
	}

	node := &ast.SwitchStatement{
		Discriminant: discriminant,
		Default:      -1,
	}

	inSwitch := self.scope.inSwitch
	self.scope.inSwitch = true
	defer func() {
		self.scope.inSwitch = inSwitch
	}()

	for index := 0; self.token != token.EOF; index++ {
		if self.token == token.RIGHT_BRACE {
			self.next()
			break
		}

		clause, err := self.parseCaseStatement()
		if err != nil {
			return nil, err
		}
		if clause.Test == nil {
			if node.Default != -1 {
				return nil, self.error(clause.Case, "Already saw a default in switch")
			}
			node.Default = index
		}
		node.Body = append(node.Body, clause)
	}

	return node, nil
}

func (self *_parser) parseWithStatement() (ast.Statement, error) {
	if _, err := self.expect(token.WITH); err != nil {
		return nil, err
	}
	if _, err := self.expect(token.LEFT_PARENTHESIS); err != nil {
		return nil, err
	}

	obj, err := self.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := self.expect(token.RIGHT_PARENTHESIS); err != nil {
		return nil, err
	}

	body, err := self.parseStatement()
	if err != nil {
		return nil, err
	}
	return &ast.WithStatement{
		Object: obj,
		Body:   body,
	}, nil
}

func (self *_parser) parseCaseStatement() (*ast.CaseStatement, error) {

	node := &ast.CaseStatement{
		Case: self.idx,
	}
	if self.token == token.DEFAULT {
		self.next()
	} else {
		if _, err := self.expect(token.CASE); err != nil {
			return nil, err
		}
		exp, err := self.parseExpression()
		if err != nil {
			return nil, err
		}
		node.Test = exp
	}
	if _, err := self.expect(token.COLON); err != nil {
		return nil, err
	}

	for {
		if self.token == token.EOF ||
			self.token == token.RIGHT_BRACE ||
			self.token == token.CASE ||
			self.token == token.DEFAULT {
			break
		}
		stmt, err := self.parseStatement()
		if err != nil {
			return nil, err
		}
		node.Consequent = append(node.Consequent, stmt)

	}

	return node, nil
}

func (self *_parser) parseIterationStatement() (ast.Statement, error) {
	inIteration := self.scope.inIteration
	self.scope.inIteration = true
	defer func() {
		self.scope.inIteration = inIteration
	}()
	return self.parseStatement()
}

func (self *_parser) parseForIn(into ast.Expression) (*ast.ForInStatement, error) {

	// Already have consumed "<into> in"

	source, err := self.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := self.expect(token.RIGHT_PARENTHESIS); err != nil {
		return nil, err
	}

	iter, err := self.parseIterationStatement()
	if err != nil {
		return nil, err
	}
	return &ast.ForInStatement{
		Into:   into,
		Source: source,
		Body:   iter,
	}, nil
}

func (self *_parser) parseFor(initializer ast.Expression) (*ast.ForStatement, error) {

	// Already have consumed "<initializer> ;"

	var test, update ast.Expression
	var err error

	if self.token != token.SEMICOLON {
		test, err = self.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	if _, err := self.expect(token.SEMICOLON); err != nil {
		return nil, err
	}

	if self.token != token.RIGHT_PARENTHESIS {
		update, err = self.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	if _, err := self.expect(token.RIGHT_PARENTHESIS); err != nil {
		return nil, err
	}

	iter, err := self.parseIterationStatement()
	if err != nil {
		return nil, err
	}
	return &ast.ForStatement{
		Initializer: initializer,
		Test:        test,
		Update:      update,
		Body:        iter,
	}, nil
}

func (self *_parser) parseForOrForInStatement() (ast.Statement, error) {
	idx, err := self.expect(token.FOR)
	if err != nil {
		return nil, err
	}
	if _, err := self.expect(token.LEFT_PARENTHESIS); err != nil {
		return nil, err
	}

	var left []ast.Expression

	forIn := false
	if self.token != token.SEMICOLON {

		allowIn := self.scope.allowIn
		self.scope.allowIn = false
		if self.token == token.VAR {
			varPos := self.idx
			self.next()
			list, err := self.parseVariableDeclarationList(varPos)
			if err != nil {
				return nil, err
			}
			if len(list) == 1 && self.token == token.IN {
				self.next() // in
				forIn = true
				left = []ast.Expression{list[0]} // There is only one declaration
			} else {
				left = list
			}
		} else {
			exp, err := self.parseExpression()
			if err != nil {
				return nil, err
			}
			left = append(left, exp)
			if self.token == token.IN {
				self.next()
				forIn = true
			}
		}
		self.scope.allowIn = allowIn
	}

	if forIn {
		switch left[0].(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression, *ast.VariableExpression:
			// These are all acceptable
		default:
			return nil, self.error(idx, "Invalid left-hand side in for-in")
		}
		return self.parseForIn(left[0])
	}

	if _, err := self.expect(token.SEMICOLON); err != nil {
		return nil, err
	}
	return self.parseFor(&ast.SequenceExpression{Sequence: left})
}

func (self *_parser) parseVariableStatement() (*ast.VariableStatement, error) {

	idx, err := self.expect(token.VAR)
	if err != nil {
		return nil, err
	}

	list, err := self.parseVariableDeclarationList(idx)
	if err != nil {
		return nil, err
	}

	if err := self.semicolon(); err != nil {
		return nil, err
	}

	return &ast.VariableStatement{
		Var:  idx,
		List: list,
	}, nil
}

func (self *_parser) parseDoWhileStatement() (ast.Statement, error) {
	inIteration := self.scope.inIteration
	self.scope.inIteration = true
	defer func() {
		self.scope.inIteration = inIteration
	}()

	if _, err := self.expect(token.DO); err != nil {
		return nil, err
	}

	var body ast.Statement
	var err error
	if self.token == token.LEFT_BRACE {
		body, err = self.parseBlockStatement()
		if err != nil {
			return nil, err
		}
	} else {
		body, err = self.parseStatement()
		if err != nil {
			return nil, err
		}
	}

	if _, err := self.expect(token.WHILE); err != nil {
		return nil, err
	}
	if _, err := self.expect(token.LEFT_PARENTHESIS); err != nil {
		return nil, err
	}
	var test ast.Expression
	test, err = self.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := self.expect(token.RIGHT_PARENTHESIS); err != nil {
		return nil, err
	}

	return &ast.DoWhileStatement{
		Body: body,
		Test: test,
	}, nil
}

func (self *_parser) parseWhileStatement() (ast.Statement, error) {
	if _, err := self.expect(token.WHILE); err != nil {
		return nil, err
	}
	if _, err := self.expect(token.LEFT_PARENTHESIS); err != nil {
		return nil, err
	}

	test, err := self.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := self.expect(token.RIGHT_PARENTHESIS); err != nil {
		return nil, err
	}

	body, err := self.parseIterationStatement()
	if err != nil {
		return nil, err
	}

	return &ast.WhileStatement{
		Test: test,
		Body: body,
	}, nil
}

func (self *_parser) parseIfStatement() (ast.Statement, error) {
	if _, err := self.expect(token.IF); err != nil {
		return nil, err
	}
	if _, err := self.expect(token.LEFT_PARENTHESIS); err != nil {
		return nil, err
	}
	exp, err := self.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := self.expect(token.RIGHT_PARENTHESIS); err != nil {
		return nil, err
	}
	var consequent ast.Statement
	if self.token == token.LEFT_BRACE {
		consequent, err = self.parseBlockStatement()
		if err != nil {
			return nil, err
		}
	} else {
		consequent, err = self.parseStatement()
		if err != nil {
			return nil, err
		}
	}

	var alternate ast.Statement
	if self.token == token.ELSE {
		self.next()
		alternate, err = self.parseStatement()
		if err != nil {
			return nil, err
		}
	}

	return &ast.IfStatement{
		Test:       exp,
		Consequent: consequent,
		Alternate:  alternate,
	}, nil
}

func (self *_parser) parseSourceElement() (ast.Statement, error) {

	return self.parseStatement()
}

func (self *_parser) parseSourceElements() ([]ast.Statement, error) {
	body := []ast.Statement(nil)

	for {
		if self.token != token.STRING {
			break
		}

		src, err := self.parseSourceElement()
		if err != nil {
			return nil, err
		}
		body = append(body, src)
	}

	for self.token != token.EOF {
		src, err := self.parseSourceElement()
		if err != nil {
			return nil, err
		}
		body = append(body, src)
	}

	return body, nil
}

func (self *_parser) parseProgram() (*ast.Program, error) {
	self.openScope()
	defer self.closeScope()
	srcElems, err := self.parseSourceElements()
	if err != nil {
		return nil, err
	}
	return &ast.Program{
		Body:            srcElems,
		DeclarationList: self.scope.declarationList,
		File:            self.file,
	}, nil
}

func (self *_parser) parseBreakStatement() (ast.Statement, error) {
	idx, err := self.expect(token.BREAK)
	if err != nil {
		return nil, err
	}
	semicolon := self.implicitSemicolon
	if self.token == token.SEMICOLON {
		semicolon = true
		self.next()
	}

	if semicolon || self.token == token.RIGHT_BRACE {
		self.implicitSemicolon = false
		if !self.scope.inIteration && !self.scope.inSwitch {
			goto illegal
		}
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.BREAK,
		}, nil
	}

	if self.token == token.IDENTIFIER {
		identifier, err := self.parseIdentifier()
		if err != nil {
			return nil, err
		}
		if !self.scope.hasLabel(identifier.Name) {
			return nil, self.error(idx, "Undefined label '%s'", identifier.Name)
		}
		if err := self.semicolon(); err != nil {
			return nil, err
		}
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.BREAK,
			Label: identifier,
		}, nil
	}

	if _, err := self.expect(token.IDENTIFIER); err != nil {
		return nil, err
	}

illegal:
	return nil, self.error(idx, "Illegal break statement")
}

func (self *_parser) parseContinueStatement() (ast.Statement, error) {
	idx, err := self.expect(token.CONTINUE)
	if err != nil {
		return nil, err
	}
	semicolon := self.implicitSemicolon
	if self.token == token.SEMICOLON {
		semicolon = true
		self.next()
	}

	if semicolon || self.token == token.RIGHT_BRACE {
		self.implicitSemicolon = false
		if !self.scope.inIteration {
			goto illegal
		}
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.CONTINUE,
		}, nil
	}

	if self.token == token.IDENTIFIER {
		identifier, err := self.parseIdentifier()
		if err != nil {
			return nil, err
		}
		if !self.scope.hasLabel(identifier.Name) {
			return nil, self.error(idx, "Undefined label '%s'", identifier.Name)
		}
		if !self.scope.inIteration {
			goto illegal
		}
		if err := self.semicolon(); err != nil {
			return nil, err
		}
		return &ast.BranchStatement{
			Idx:   idx,
			Token: token.CONTINUE,
			Label: identifier,
		}, nil
	}

	if _, err := self.expect(token.IDENTIFIER); err != nil {
		return nil, err
	}

illegal:
	return nil, self.error(idx, "Illegal continue statement")
}

// Find the next statement after an error (recover)
func (self *_parser) nextStatement() {
	for {
		switch self.token {
		case token.BREAK, token.CONTINUE,
			token.FOR, token.IF, token.RETURN, token.SWITCH,
			token.VAR, token.DO, token.TRY, token.WITH,
			token.WHILE, token.THROW, token.CATCH, token.FINALLY:
			// Return only if parser made some progress since last
			// sync or if it has not reached 10 next calls without
			// progress. Otherwise consume at least one token to
			// avoid an endless parser loop
			if self.idx == self.recover.idx && self.recover.count < 10 {
				self.recover.count++
				return
			}
			if self.idx > self.recover.idx {
				self.recover.idx = self.idx
				self.recover.count = 0
				return
			}
			// Reaching here indicates a parser bug, likely an
			// incorrect token list in this function, but it only
			// leads to skipping of possibly correct code if a
			// previous error is present, and thus is preferred
			// over a non-terminating parse.
		case token.EOF:
			return
		}
		self.next()
	}
}
