package parser

import (
	"fmt"
	"github.com/mamaar/risotto/ast"
	"github.com/mamaar/risotto/file"
	"github.com/mamaar/risotto/token"
)

func (self *_parser) parseIdentifier() (*ast.Identifier, error) {
	literal := self.literal
	idx := self.idx
	self.next()
	return &ast.Identifier{
		Name: literal,
		Idx:  idx,
	}, nil
}

func (self *_parser) parsePrimaryExpression() (ast.Expression, error) {
	literal := self.literal
	idx := self.idx

	switch self.token {
	case token.LESS:
		return self.parseJSXElement()
	case token.IDENTIFIER:
		self.next()
		if len(literal) > 1 {
			tkn, strict := token.IsKeyword(literal)
			if tkn == token.KEYWORD {
				if !strict {
					return nil, self.error(idx, "Unexpected reserved word")
				}
			}
		}
		return &ast.Identifier{
			Name: literal,
			Idx:  idx,
		}, nil
	case token.NULL:
		self.next()
		return &ast.NullLiteral{
			Idx:     idx,
			Literal: literal,
		}, nil
	case token.BOOLEAN:
		self.next()
		value := false
		switch literal {
		case "true":
			value = true
		case "false":
			value = false
		default:
			return nil, self.error(idx, "Illegal boolean literal")
		}
		return &ast.BooleanLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   value,
		}, nil
	case token.STRING:
		self.next()
		value, err := parseStringLiteral(literal[1 : len(literal)-1])
		if err != nil {
			return nil, self.error(idx, err.Error())
		}
		return &ast.StringLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   value,
		}, nil
	case token.NUMBER:
		self.next()
		value, err := parseNumberLiteral(literal)
		if err != nil {
			return nil, self.error(idx, err.Error())
		}
		return &ast.NumberLiteral{
			Idx:     idx,
			Literal: literal,
			Value:   value,
		}, nil
	case token.SLASH, token.QUOTIENT_ASSIGN:
		return self.parseRegExpLiteral()
	case token.LEFT_BRACE:
		return self.parseObjectLiteral()
	case token.LEFT_BRACKET:
		return self.parseArrayLiteral()
	case token.LEFT_PARENTHESIS:
		_, err := self.expect(token.LEFT_PARENTHESIS)
		if err != nil {
			return nil, err
		}
		expression, err := self.parseExpression()
		if err != nil {
			return nil, err
		}
		_, err = self.expect(token.RIGHT_PARENTHESIS)
		if err != nil {
			return nil, err
		}
		return expression, nil
	case token.THIS:
		self.next()
		return &ast.ThisExpression{
			Idx: idx,
		}, nil
	case token.FUNCTION:
		f, err := self.parseFunction(false)
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	err := self.errorUnexpectedToken(self.token)
	return nil, err
}

func (self *_parser) parseRegExpLiteral() (*ast.RegExpLiteral, error) {

	offset := self.chrOffset - 1 // Opening slash already gotten
	if self.token == token.QUOTIENT_ASSIGN {
		offset-- // =
	}
	idx := self.idxOf(offset)

	pattern, err := self.scanString(offset)
	endOffset := self.chrOffset

	self.next()
	if err == nil {
		pattern = pattern[1 : len(pattern)-1]
	}

	flags := ""
	if self.token == token.IDENTIFIER { // gim

		flags = self.literal
		self.next()
		endOffset = self.chrOffset - 1
	}

	var value string
	literal := self.str[offset:endOffset]

	return &ast.RegExpLiteral{
		Idx:     idx,
		Literal: literal,
		Pattern: pattern,
		Flags:   flags,
		Value:   value,
	}, nil
}

func (self *_parser) parseVariableDeclaration(declarationList *[]*ast.VariableExpression) (ast.Expression, error) {

	if self.token != token.IDENTIFIER {
		if _, err := self.expect(token.IDENTIFIER); err != nil {
			return nil, err
		}

	}

	literal := self.literal
	idx := self.idx
	self.next()
	node := &ast.VariableExpression{
		Name: literal,
		Idx:  idx,
	}

	if declarationList != nil {
		*declarationList = append(*declarationList, node)
	}

	if self.token == token.ASSIGN {
		self.next()
		init, err := self.parseAssignmentExpression()
		if err != nil {
			return nil, err
		}
		node.Initializer = init
	}

	return node, nil
}

func (self *_parser) parseVariableDeclarationList(var_ file.Idx) ([]ast.Expression, error) {

	var declarationList []*ast.VariableExpression // Avoid bad expressions
	var list []ast.Expression

	for {
		decl, err := self.parseVariableDeclaration(&declarationList)
		if err != nil {
			return nil, err
		}
		list = append(list, decl)
		if self.token != token.COMMA {
			break
		}
		self.next()
	}

	self.scope.declare(&ast.VariableDeclaration{
		Var:  var_,
		List: declarationList,
	})

	return list, nil
}

func (self *_parser) parseObjectPropertyKey() (string, string) {
	idx, tkn, literal := self.idx, self.token, self.literal
	value := ""
	self.next()
	switch tkn {
	case token.IDENTIFIER:
		value = literal
	case token.NUMBER:
		var err error
		_, err = parseNumberLiteral(literal)
		if err != nil {
			self.error(idx, err.Error())
		} else {
			value = literal
		}
	case token.STRING:
		var err error
		value, err = parseStringLiteral(literal[1 : len(literal)-1])
		if err != nil {
			self.error(idx, err.Error())
		}
	default:
		// null, false, class, etc.
		if matchIdentifier.MatchString(literal) {
			value = literal
		}
	}
	return literal, value
}

func (self *_parser) parseObjectProperty() (ast.Property, error) {

	literal, value := self.parseObjectPropertyKey()
	if literal == "get" && self.token != token.COLON {
		idx := self.idx
		_, value := self.parseObjectPropertyKey()
		parameterList, err := self.parseFunctionParameterList()
		if err != nil {
			return ast.Property{}, err
		}

		node := &ast.FunctionLiteral{
			Function:      idx,
			ParameterList: parameterList,
		}
		self.parseFunctionBlock(node)
		return ast.Property{
			Key:   value,
			Kind:  "get",
			Value: node,
		}, nil
	} else if literal == "set" && self.token != token.COLON {
		idx := self.idx
		_, value := self.parseObjectPropertyKey()
		parameterList, err := self.parseFunctionParameterList()
		if err != nil {
			return ast.Property{}, err
		}

		node := &ast.FunctionLiteral{
			Function:      idx,
			ParameterList: parameterList,
		}
		self.parseFunctionBlock(node)
		return ast.Property{
			Key:   value,
			Kind:  "set",
			Value: node,
		}, nil
	}

	if _, err := self.expect(token.COLON); err != nil {
		return ast.Property{}, err
	}

	val, err := self.parseAssignmentExpression()
	if err != nil {
		return ast.Property{}, err
	}
	return ast.Property{
		Key:   value,
		Kind:  "value",
		Value: val,
	}, nil
}

func (self *_parser) parseObjectLiteral() (ast.Expression, error) {
	var value []ast.Property
	idx0, err := self.expect(token.LEFT_BRACE)
	if err != nil {
		return nil, err
	}
	for self.token != token.RIGHT_BRACE && self.token != token.EOF {
		property, err := self.parseObjectProperty()
		if err != nil {
			return nil, err
		}
		value = append(value, property)
		if self.token == token.COMMA {
			self.next()
			continue
		}
	}
	idx1, err := self.expect(token.RIGHT_BRACE)
	if err != nil {
		return nil, err
	}

	return &ast.ObjectLiteral{
		LeftBrace:  idx0,
		RightBrace: idx1,
		Value:      value,
	}, nil
}

func (self *_parser) parseArrayLiteral() (ast.Expression, error) {

	idx0, err := self.expect(token.LEFT_BRACKET)
	if err != nil {
		return nil, err
	}
	var value []ast.Expression
	for self.token != token.RIGHT_BRACKET && self.token != token.EOF {
		if self.token == token.COMMA {
			self.next()
			value = append(value, nil)
			continue
		}

		val, err := self.parseAssignmentExpression()
		if err != nil {
			return nil, err
		}
		value = append(value, val)
		if self.token != token.RIGHT_BRACKET {
			if _, err := self.expect(token.COMMA); err != nil {
				return nil, err
			}
		}
	}
	idx1, err := self.expect(token.RIGHT_BRACKET)
	if err != nil {
		return nil, err
	}

	return &ast.ArrayLiteral{
		LeftBracket:  idx0,
		RightBracket: idx1,
		Value:        value,
	}, nil
}

func (self *_parser) parseArgumentList() (argumentList []ast.Expression, idx0, idx1 file.Idx, err error) {
	if idx0, err = self.expect(token.LEFT_PARENTHESIS); err != nil {
		return
	}

	if self.token != token.RIGHT_PARENTHESIS {
		for {
			var val ast.Expression
			var e error
			if val, e = self.parseAssignmentExpression(); e != nil {
				err = e
				return
			}
			argumentList = append(argumentList, val)
			if self.token != token.COMMA {
				break
			}
			self.next()
		}
	}
	idx1, err = self.expect(token.RIGHT_PARENTHESIS)
	return
}

func (self *_parser) parseCallExpression(left ast.Expression) (ast.Expression, error) {
	argumentList, idx0, idx1, err := self.parseArgumentList()
	if err != nil {
		return nil, err
	}

	// If calling require function with a parameter
	if module, ok := self.isRequireModule(left, argumentList); self.parseModular && ok {
		mPath, ok := self.resolvePath(module)
		if !ok {
			return nil, fmt.Errorf("Could not open module '%s' from '%s'", mPath, self.filepath)
		}
		if _, ok := self.rootModule.Dependencies[mPath]; !ok {
			popts := ParserOptions{
				FileName:          mPath,
				ModulesLookupDirs: self.modulesLookupDirs,
				ParseModular:      true,
			}

			parser, err := NewParser(popts)
			if err != nil {
				return nil, err
			}
			parsedModule, err := parser.ParseModule()
			if err != nil {
				return nil, err
			}
			self.rootModule.Dependencies[module] = parsedModule
			self.isModular = true
		}
	}

	return &ast.CallExpression{
		Callee:           left,
		LeftParenthesis:  idx0,
		ArgumentList:     argumentList,
		RightParenthesis: idx1,
	}, nil
}

func (self *_parser) parseDotMember(left ast.Expression) (ast.Expression, error) {
	if _, err := self.expect(token.PERIOD); err != nil {
		return nil, err
	}

	literal := self.literal
	idx := self.idx

	if !matchIdentifier.MatchString(literal) {
		if _, err := self.expect(token.IDENTIFIER); err != nil {
			return nil, err
		}
	}

	self.next()

	return &ast.DotExpression{
		Left: left,
		Identifier: ast.Identifier{
			Idx:  idx,
			Name: literal,
		},
	}, nil
}

func (self *_parser) parseBracketMember(left ast.Expression) (ast.Expression, error) {
	idx0, err := self.expect(token.LEFT_BRACKET)
	if err != nil {
		return nil, err
	}

	member, err := self.parseExpression()
	if err != nil {
		return nil, err
	}

	idx1, err := self.expect(token.RIGHT_BRACKET)
	if err != nil {
		return nil, err
	}
	return &ast.BracketExpression{
		LeftBracket:  idx0,
		Left:         left,
		Member:       member,
		RightBracket: idx1,
	}, nil
}

func (self *_parser) parseNewExpression() (ast.Expression, error) {
	idx, err := self.expect(token.NEW)
	if err != nil {
		return nil, err
	}
	callee, err := self.parseLeftHandSideExpression()
	if err != nil {
		return nil, err
	}
	node := &ast.NewExpression{
		New:    idx,
		Callee: callee,
	}
	if self.token == token.LEFT_PARENTHESIS {
		argumentList, idx0, idx1, err := self.parseArgumentList()
		if err != nil {
			return nil, err
		}
		node.ArgumentList = argumentList
		node.LeftParenthesis = idx0
		node.RightParenthesis = idx1
	}
	return node, nil
}

func (self *_parser) parseLeftHandSideExpression() (ast.Expression, error) {

	var left ast.Expression
	var err error
	if self.token == token.NEW {
		left, err = self.parseNewExpression()
		if err != nil {
			return nil, err
		}
	} else {
		left, err = self.parsePrimaryExpression()
		if err != nil {
			return nil, err
		}
	}

	for {
		if self.token == token.PERIOD {
			left, err = self.parseDotMember(left)
			if err != nil {
				return nil, err
			}
		} else if self.token == token.LEFT_BRACE {
			left, err = self.parseBracketMember(left)
			if err != nil {
				return nil, err
			}
		} else {
			break
		}
	}

	return left, nil
}

func (self *_parser) parseLeftHandSideExpressionAllowCall() (ast.Expression, error) {

	allowIn := self.scope.allowIn
	self.scope.allowIn = true
	defer func() {
		self.scope.allowIn = allowIn
	}()

	var left ast.Expression
	var err error
	if self.token == token.NEW {
		left, err = self.parseNewExpression()
		if err != nil {
			return nil, err
		}
	} else {
		left, err = self.parsePrimaryExpression()
		if err != nil {
			return nil, err
		}
	}

	for {
		var err error
		if self.token == token.PERIOD {
			left, err = self.parseDotMember(left)
		} else if self.token == token.LEFT_BRACKET {
			left, err = self.parseBracketMember(left)
		} else if self.token == token.LEFT_PARENTHESIS {
			left, err = self.parseCallExpression(left)
		} else {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return left, nil
}

func (self *_parser) parsePostfixExpression() (ast.Expression, error) {
	operand, err := self.parseLeftHandSideExpressionAllowCall()
	if err != nil {
		return nil, err
	}

	switch self.token {
	case token.INCREMENT, token.DECREMENT:
		// Make sure there is no line terminator here
		if self.implicitSemicolon {
			break
		}
		tkn := self.token
		idx := self.idx
		self.next()
		switch operand.(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression:
		default:
			return nil, self.error(idx, "Invalid left-hand side in assignment")
		}
		return &ast.UnaryExpression{
			Operator: tkn,
			Idx:      idx,
			Operand:  operand,
			Postfix:  true,
		}, nil
	}

	return operand, nil
}

func (self *_parser) parseUnaryExpression() (ast.Expression, error) {

	switch self.token {
	case token.PLUS, token.MINUS, token.NOT, token.BITWISE_NOT:
		fallthrough
	case token.DELETE, token.VOID, token.TYPEOF:
		tkn := self.token
		idx := self.idx
		self.next()
		val, err := self.parseUnaryExpression()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpression{
			Operator: tkn,
			Idx:      idx,
			Operand:  val,
		}, nil
	case token.INCREMENT, token.DECREMENT:
		tkn := self.token
		idx := self.idx
		self.next()
		operand, err := self.parseUnaryExpression()
		if err != nil {
			return nil, err
		}
		switch operand.(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression:
		default:
			return nil, self.error(idx, "Invalid left-hand side in assignment")
		}
		return &ast.UnaryExpression{
			Operator: tkn,
			Idx:      idx,
			Operand:  operand,
		}, nil
	}

	return self.parsePostfixExpression()
}

func (self *_parser) parseMultiplicativeExpression() (ast.Expression, error) {
	next := self.parseUnaryExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	for self.token == token.MULTIPLY || self.token == token.SLASH ||
		self.token == token.REMAINDER {
		tkn := self.token
		self.next()
		val, err := next()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    val,
		}
	}

	return left, nil
}

func (self *_parser) parseAdditiveExpression() (ast.Expression, error) {
	next := self.parseMultiplicativeExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	for self.token == token.PLUS || self.token == token.MINUS {
		tkn := self.token
		self.next()
		val, err := next()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    val,
		}
	}

	return left, nil
}

func (self *_parser) parseShiftExpression() (ast.Expression, error) {
	next := self.parseAdditiveExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	for self.token == token.SHIFT_LEFT || self.token == token.SHIFT_RIGHT ||
		self.token == token.UNSIGNED_SHIFT_RIGHT {
		tkn := self.token
		self.next()
		val, err := next()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    val,
		}
	}

	return left, nil
}

func (self *_parser) parseRelationalExpression() (ast.Expression, error) {
	next := self.parseShiftExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	allowIn := self.scope.allowIn
	self.scope.allowIn = true
	defer func() {
		self.scope.allowIn = allowIn
	}()

	switch self.token {
	case token.LESS, token.LESS_OR_EQUAL, token.GREATER, token.GREATER_OR_EQUAL:
		tkn := self.token
		self.next()
		r, err := self.parseRelationalExpression()
		if err != nil {
			return nil, err
		}
		return &ast.BinaryExpression{
			Operator:   tkn,
			Left:       left,
			Right:      r,
			Comparison: true,
		}, nil
	case token.INSTANCEOF:
		tkn := self.token
		self.next()
		r, err := self.parseRelationalExpression()
		if err != nil {
			return nil, err
		}
		return &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    r,
		}, nil
	case token.IN:
		if !allowIn {
			return left, nil
		}
		tkn := self.token
		self.next()
		r, err := self.parseRelationalExpression()
		if err != nil {
			return nil, err
		}
		return &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    r,
		}, nil
	}

	return left, nil
}

func (self *_parser) parseEqualityExpression() (ast.Expression, error) {
	next := self.parseRelationalExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	for self.token == token.EQUAL || self.token == token.NOT_EQUAL ||
		self.token == token.STRICT_EQUAL || self.token == token.STRICT_NOT_EQUAL {
		tkn := self.token
		self.next()
		val, err := next()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Operator:   tkn,
			Left:       left,
			Right:      val,
			Comparison: true,
		}
	}

	return left, nil
}

func (self *_parser) parseBitwiseAndExpression() (ast.Expression, error) {
	next := self.parseEqualityExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	for self.token == token.AND {
		tkn := self.token
		self.next()
		val, err := next()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    val,
		}
	}

	return left, nil
}

func (self *_parser) parseBitwiseExclusiveOrExpression() (ast.Expression, error) {
	next := self.parseBitwiseAndExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	for self.token == token.EXCLUSIVE_OR {
		tkn := self.token
		self.next()
		val, err := next()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    val,
		}
	}

	return left, nil
}

func (self *_parser) parseBitwiseOrExpression() (ast.Expression, error) {
	next := self.parseBitwiseExclusiveOrExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	for self.token == token.OR {
		tkn := self.token
		self.next()
		val, err := next()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    val,
		}
	}

	return left, nil
}

func (self *_parser) parseLogicalAndExpression() (ast.Expression, error) {
	next := self.parseBitwiseOrExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	for self.token == token.LOGICAL_AND {
		tkn := self.token
		self.next()
		val, err := next()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    val,
		}
	}

	return left, nil
}

func (self *_parser) parseLogicalOrExpression() (ast.Expression, error) {
	next := self.parseLogicalAndExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	for self.token == token.LOGICAL_OR {
		tkn := self.token
		self.next()
		val, err := next()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Operator: tkn,
			Left:     left,
			Right:    val,
		}
	}

	return left, nil
}

func (self *_parser) parseConditionlExpression() (ast.Expression, error) {
	left, err := self.parseLogicalOrExpression()
	if err != nil {
		return nil, err
	}

	if self.token == token.QUESTION_MARK {
		self.next()
		consequent, err := self.parseAssignmentExpression()
		if err != nil {
			return nil, err
		}
		if _, err := self.expect(token.COLON); err != nil {
			return nil, err
		}

		val, err := self.parseAssignmentExpression()
		if err != nil {
			return nil, err
		}
		return &ast.ConditionalExpression{
			Test:       left,
			Consequent: consequent,
			Alternate:  val,
		}, nil
	}

	return left, nil
}

func (self *_parser) parseAssignmentExpression() (ast.Expression, error) {
	left, err := self.parseConditionlExpression()
	if err != nil {
		return nil, err
	}
	var operator token.Token
	switch self.token {
	case token.ASSIGN:
		operator = self.token
	case token.ADD_ASSIGN:
		operator = token.PLUS
	case token.SUBTRACT_ASSIGN:
		operator = token.MINUS
	case token.MULTIPLY_ASSIGN:
		operator = token.MULTIPLY
	case token.QUOTIENT_ASSIGN:
		operator = token.SLASH
	case token.REMAINDER_ASSIGN:
		operator = token.REMAINDER
	case token.AND_ASSIGN:
		operator = token.AND
	case token.AND_NOT_ASSIGN:
		operator = token.AND_NOT
	case token.OR_ASSIGN:
		operator = token.OR
	case token.EXCLUSIVE_OR_ASSIGN:
		operator = token.EXCLUSIVE_OR
	case token.SHIFT_LEFT_ASSIGN:
		operator = token.SHIFT_LEFT
	case token.SHIFT_RIGHT_ASSIGN:
		operator = token.SHIFT_RIGHT
	case token.UNSIGNED_SHIFT_RIGHT_ASSIGN:
		operator = token.UNSIGNED_SHIFT_RIGHT
	}

	if operator != 0 {
		self.next()
		switch left.(type) {
		case *ast.Identifier, *ast.DotExpression, *ast.BracketExpression:
		default:
			return nil, self.error(left.Idx0(), "Invalid left-hand side in assignment")
		}
		val, err := self.parseAssignmentExpression()
		if err != nil {
			return nil, err
		}
		return &ast.AssignExpression{
			Left:     left,
			Operator: operator,
			Right:    val,
		}, nil
	}

	return left, nil
}

func (self *_parser) parseExpression() (ast.Expression, error) {
	next := self.parseAssignmentExpression
	left, err := next()
	if err != nil {
		return nil, err
	}

	if self.token == token.COMMA {
		sequence := []ast.Expression{left}
		for {
			if self.token != token.COMMA {
				break
			}
			self.next()
			val, err := next()
			if err != nil {
				return nil, err
			}
			sequence = append(sequence, val)
		}
		return &ast.SequenceExpression{
			Sequence: sequence,
		}, nil
	}

	return left, nil
}
