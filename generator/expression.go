package generator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/walesey/go-bundle/ast"
	"github.com/walesey/go-bundle/token"
)

func (g *generator) generateExpression(exp ast.Expression) error {
	g.descentExpression()
	defer g.ascentExpression()
	switch exp.(type) {
	case *ast.JSXExpression:
		return g.jsxExpression(exp.(*ast.JSXExpression))
	case *ast.JSXText:
		return g.jsxText(exp.(*ast.JSXText))
	case *ast.JSXBlock:
		return g.jsxBlockExpression(exp.(*ast.JSXBlock))
	case *ast.VariableExpression:
		return g.variableExpression(exp.(*ast.VariableExpression))
	case *ast.FunctionLiteral:
		return g.functionLiteral(exp.(*ast.FunctionLiteral), true)
	case *ast.ObjectLiteral:
		return g.objectLiteral(exp.(*ast.ObjectLiteral))
	case *ast.NumberLiteral:
		return g.numberLiteral(exp.(*ast.NumberLiteral))
	case *ast.StringLiteral:
		return g.stringLiteral(exp.(*ast.StringLiteral))
	case *ast.ArrayLiteral:
		return g.arrayLiteral(exp.(*ast.ArrayLiteral))
	case *ast.BooleanLiteral:
		return g.booleanLiteral(exp.(*ast.BooleanLiteral))
	case *ast.RegExpLiteral:
		return g.regExpLiteral(exp.(*ast.RegExpLiteral))
	case *ast.NullLiteral:
		return g.nullLiteral(exp.(*ast.NullLiteral))
	case *ast.Identifier:
		return g.identifier(exp.(*ast.Identifier))
	case *ast.UnaryExpression:
		return g.unaryExpression(exp.(*ast.UnaryExpression))
	case *ast.BinaryExpression:
		return g.binaryExpression(exp.(*ast.BinaryExpression))
	case *ast.CallExpression:
		return g.callExpression(exp.(*ast.CallExpression))
	case *ast.DotExpression:
		return g.dotExpression(exp.(*ast.DotExpression))
	case *ast.AssignExpression:
		return g.assignExpression(exp.(*ast.AssignExpression))
	case *ast.ConditionalExpression:
		return g.conditionalExpression(exp.(*ast.ConditionalExpression))
	case *ast.NewExpression:
		return g.newExpression(exp.(*ast.NewExpression))
	case *ast.ThisExpression:
		return g.thisExpression(exp.(*ast.ThisExpression))
	case *ast.BracketExpression:
		return g.bracketExpression(exp.(*ast.BracketExpression))
	case *ast.SequenceExpression:
		return g.sequenceExpression(exp.(*ast.SequenceExpression))
	case *ast.DynamicStringExpression:
		return g.dynamicStringExpression(exp.(*ast.DynamicStringExpression))
	case nil:
		return nil
	default:
		return fmt.Errorf("Expression is not implemented: <%v>", reflect.TypeOf(exp))
	}
}

func (g *generator) jsxExpression(jsx *ast.JSXExpression) error {
	g.generateExpression(jsx.Identifier)
	return nil
}

func (g *generator) jsxText(jsx *ast.JSXText) error {
	trimmed := jsx.Literal
	if idx := strings.Index(trimmed, "\n"); idx != -1 {
		trimmed = trimmed[0:idx]
	}

	g.write("\"")
	g.write(trimmed)
	g.write("\"")
	return nil
}

func (g *generator) jsxBlockExpression(jsx *ast.JSXBlock) error {
	g.write("React.createElement(")

	openingElement := jsx.OpeningElement
	elementName := openingElement.Name.Name
	if rune(elementName[0]) >= 'a' && rune(elementName[0]) <= 'z' {
		g.write(escapeKey(elementName))
	} else {
		g.write(elementName)
	}
	g.write(", ")

	properties := openingElement.PropertyList
	if len(properties) > 0 {
		propObject := &ast.ObjectLiteral{Value: properties}
		if err := g.objectLiteral(propObject); err != nil {
			return err
		}
	} else {
		g.write("null")
	}

	for _, c := range jsx.Body {
		g.write(", ")
		g.generateExpression(c)
	}

	g.write(")")

	return nil
}

func (g *generator) sequenceExpression(s *ast.SequenceExpression) error {
	if g.isInExpression() {
		g.write("(")
	}
	for i, e := range s.Sequence {
		if err := g.generateExpression(e); err != nil {
			return err
		}
		if i < len(s.Sequence)-1 {
			g.write(", ")
		}
	}
	if g.isInExpression() {
		g.write(")")
	}

	return nil
}

func (g *generator) bracketExpression(b *ast.BracketExpression) error {
	if err := g.generateExpression(b.Left); err != nil {
		return err
	}
	g.write("[")
	if err := g.generateExpression(b.Member); err != nil {
		return err
	}
	g.write("]")
	return nil
}

func (g *generator) thisExpression(t *ast.ThisExpression) error {
	g.write("this")
	return nil
}

func (g *generator) newExpression(n *ast.NewExpression) error {
	g.write("new ")
	if err := g.generateExpression(n.Callee); err != nil {
		return err
	}
	return g.argumentList(n.ArgumentList)
}

func (g *generator) conditionalExpression(c *ast.ConditionalExpression) error {
	if err := g.generateExpression(c.Test); err != nil {
		return err
	}
	g.write(" ? ")
	if err := g.generateExpression(c.Consequent); err != nil {
		return err
	}
	g.write(" : ")
	if err := g.generateExpression(c.Alternate); err != nil {
		return err
	}
	return nil
}

func (g *generator) assignExpression(a *ast.AssignExpression) error {
	if g.isInExpression() && !g.isInInitializer {
		g.write("(")
	}
	if err := g.generateExpression(a.Left); err != nil {
		return err
	}

	op := ""
	if a.Operator != token.ASSIGN {
		op = a.Operator.String() + op
	}
	op += token.ASSIGN.String()
	g.write(" " + op + " ")

	if err := g.generateExpression(a.Right); err != nil {
		return err
	}
	if g.isInExpression() && !g.isInInitializer {
		g.write(")")
	}

	return nil
}

func (g *generator) dotExpression(d *ast.DotExpression) error {
	if err := g.generateExpression(d.Left); err != nil {
		return err
	}

	g.write(".")

	return g.identifier(d.Identifier)
}

func (g *generator) callExpression(c *ast.CallExpression) error {
	if identifier, ok := c.Callee.(*ast.Identifier); ok {
		if g.bundle != nil && identifier.Name == "require" {
			if len(c.ArgumentList) < 1 {
				return fmt.Errorf("Not enought arguments in call to require")
			}

			requireStr, ok := c.ArgumentList[0].(*ast.StringLiteral)
			if !ok {
				return fmt.Errorf("Expected string argument in call to require")
			}

			g.write("require('")
			modulePath, err := g.bundle.resolveModule(requireStr.Value, g.filePath)
			if err == nil {
				g.write(modulePath)
			} else {
				g.write(requireStr.Value)
			}
			g.write("')")
			return nil
		}
	}

	g.isCalleeExpression = true
	if err := g.generateExpression(c.Callee); err != nil {
		return err
	}
	return g.argumentList(c.ArgumentList)
}

func (g *generator) binaryExpression(b *ast.BinaryExpression) error {
	g.write("(")
	if err := g.generateExpression(b.Left); err != nil {
		return err
	}

	g.write(" " + b.Operator.String() + " ")

	if err := g.generateExpression(b.Right); err != nil {
		return err
	}
	g.write(")")
	return nil
}

func (g *generator) unaryExpression(u *ast.UnaryExpression) error {
	if !u.Postfix {
		g.write(u.Operator.String())
		if u.Operator == token.DELETE || u.Operator == token.TYPEOF || u.Operator == token.VOID {
			g.write(" ")
		}
	}

	if err := g.generateExpression(u.Operand); err != nil {
		return err
	}

	if u.Postfix {
		g.write(u.Operator.String())
	}

	return nil
}

func (g *generator) identifier(i *ast.Identifier) error {
	g.write(i.Name)
	return nil
}

func (g *generator) nullLiteral(n *ast.NullLiteral) error {
	g.write("null")
	return nil
}

func (g *generator) regExpLiteral(r *ast.RegExpLiteral) error {
	g.write("(" + r.Literal + ")")
	return nil
}

func (g *generator) booleanLiteral(b *ast.BooleanLiteral) error {
	g.write(b.Literal)
	return nil
}

func (g *generator) arrayLiteral(a *ast.ArrayLiteral) error {
	g.write("[")
	for i, e := range a.Value {
		if err := g.generateExpression(e); err != nil {
			return err
		}
		if i < len(a.Value)-1 {
			g.write(", ")
		}
	}
	g.write("]")
	return nil
}

func (g *generator) stringLiteral(s *ast.StringLiteral) error {
	g.write(s.Literal)
	return nil
}

func (g *generator) numberLiteral(n *ast.NumberLiteral) error {
	g.write(n.Literal)
	return nil
}

func (g *generator) property(p ast.Property) error {
	if len(p.Key) > 0 {
		key := escapeKeyIfRequired(p.Key)

		g.writeIndentation(key)
		g.write(": ")
	}
	return g.generateExpression(p.Value)
}

func (g *generator) objectLiteral(o *ast.ObjectLiteral) error {
	spread := false
	for _, p := range o.Value {
		if p.Kind == "spread" {
			spread = true
		}
	}

	if spread {
		g.write("Object.assign({}, ")
		objectOpen := false
		for i, p := range o.Value {
			if p.Kind == "spread" {
				if objectOpen {
					g.write(" }")
					objectOpen = false
				}
				if i > 0 {
					g.write(", ")
				}
			} else {
				if i > 0 {
					g.write(", ")
				}
				if !objectOpen {
					g.write("{ ")
					objectOpen = true
				}
			}
			if err := g.property(p); err != nil {
				return err
			}
			if objectOpen && i == len(o.Value)-1 {
				g.write(" }")
			}
		}
		g.write(")")
	} else {
		g.write("{\n")
		g.indentLevel++
		for i, p := range o.Value {
			if err := g.property(p); err != nil {
				return err
			}
			if i < len(o.Value)-1 {
				g.write(",")
			}
			g.write("\n")
		}
		g.indentLevel--
		g.writeAlone("}")
	}
	return nil
}

func (g *generator) functionLiteral(f *ast.FunctionLiteral, newline bool) error {
	isAnonymous := f.Name == nil

	if isAnonymous {
		g.write("(function ")
		g.isCalleeExpression = false
		defer g.write(")")
	} else {
		if newline {
			g.writeLine("")
		}
		g.write("function ")
		if err := g.generateExpression(f.Name); err != nil {
			return err
		}
	}

	if err := g.parameterList(f.ParameterList); err != nil {
		return err
	}
	g.write(" ")
	return g.generateStatement(f.Body, f.DeclarationList)
}

func (g *generator) variableExpression(v *ast.VariableExpression) error {
	g.write(v.Name)

	if v.Initializer != nil {
		g.write(" = ")
	}

	if err := g.generateExpression(v.Initializer); err != nil {
		return err
	}

	return nil
}

func (g *generator) dynamicStringExpression(d *ast.DynamicStringExpression) error {
	if len(d.List) == 0 {
		g.write("''")
	} else {
		for i, e := range d.List {
			if err := g.generateExpression(e); err != nil {
				return err
			}
			if i < len(d.List)-1 {
				g.write(" + ")
			}
		}
	}

	return nil
}
