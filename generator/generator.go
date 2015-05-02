package main

import (
	"bytes"
	"fmt"
	"github.com/robertkrimen/otto/ast"
	"github.com/robertkrimen/otto/parser"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

type generator struct {
	buffer      *bytes.Buffer
	indentLevel int
	indentation string
	currentLine int
	currentChar int

	expressionLevel    int
	isInInitializer    bool
	isCalleeExpression bool
}

// Generate builds javascript from the program
// passed as an argument.
func Generate(p *ast.Program) (io.Reader, error) {
	gen := &generator{
		buffer:      &bytes.Buffer{},
		indentation: "    ",
	}

	if err := gen.generateProgram(p); err != nil {
		return nil, err
	}

	return gen.code(), nil
}

// ParseAndGenerate takes an io.Reader to be parsed and
// generate javascript code.
func ParseAndGenerate(in io.Reader) (io.Reader, error) {
	prog, err := parser.ParseFile(nil, "<input>", in, parser.IgnoreRegExpErrors)
	if err != nil {
		return nil, err
	}

	return Generate(prog)
}

func (g *generator) indentationString() string {
	return strings.Repeat(g.indentation, g.indentLevel)
}

func (g *generator) write(s string) {
	g.buffer.WriteString(s)

	g.currentLine += strings.Count(s, "\n")
	if lastIndex := strings.LastIndex(s, "\n"); lastIndex != -1 {
		g.currentChar = len(s[len("\n")+lastIndex:])
	} else {
		g.currentChar += len(s)
	}
}

func (g *generator) writeIndentation(s string) int {

	if g.currentChar > 0 && g.currentChar%len(g.indentation) == 0 {
		g.write(s)
		return len(s)
	}

	inlineIndent := len(g.indentationString()) - g.currentChar%len(g.indentation)
	indent := strings.Repeat(" ", inlineIndent)
	g.write(indent + s)
	return inlineIndent + len(s)
}

func (g *generator) writeLine(s string) {
	g.write("\n")
	g.currentLine++
	chrs := g.writeIndentation(s)
	g.currentChar = chrs
}

func (g *generator) code() io.Reader {
	return g.buffer
}

func (g *generator) generateProgram(p *ast.Program) error {
	for _, dcl := range p.DeclarationList {
		if err := g.generateDeclaration(dcl); err != nil {
			return err
		}
	}

	for _, stmt := range p.Body {
		if err := g.generateStatement(stmt, nil); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) generateDeclaration(d ast.Declaration) error {
	if fn, ok := d.(*ast.FunctionDeclaration); ok {
		return g.functionLiteral(fn.Function)
	}

	return nil
}

func (g *generator) parameterList(pl *ast.ParameterList) error {
	g.write("(")
	for i, p := range pl.List {
		if err := g.identifier(p); err != nil {
			return err
		}
		if i < len(pl.List)-1 {
			g.write(", ")
		}
	}
	g.write(")")
	return nil
}

func (g *generator) argumentList(exps []ast.Expression) error {
	g.write("(")
	for i, a := range exps {
		if err := g.generateExpression(a); err != nil {
			return err
		}
		if i < len(exps)-1 {
			g.write(", ")
		}
	}
	g.write(")")
	return nil
}

func (g *generator) isInExpression() bool {
	return g.expressionLevel > 0
}

func (g *generator) descentExpression() {
	g.expressionLevel++
}

func (g *generator) ascentExpression() {
	g.expressionLevel--
}

func escapeKey(k string) string {
	return fmt.Sprintf("\"%s\"", k)
}

func escapeKeyIfRequired(k string) string {
	if len(k) < 1 {
		return escapeKey(k)
	}
	if !isIdentifierStart(rune(k[0])) {
		return escapeKey(k)
	}
	for _, c := range k {
		if !isIdentifierPart(c) {
			return escapeKey(k)
		}
	}

	return k
}

func isIdentifierStart(chr rune) bool {
	return chr == '$' || chr == '_' || chr == '\\' ||
		'a' <= chr && chr <= 'z' || 'A' <= chr && chr <= 'Z' ||
		chr >= utf8.RuneSelf && unicode.IsLetter(chr)
}
func isIdentifierPart(chr rune) bool {
	return chr == '$' || chr == '_' || chr == '\\' ||
		'a' <= chr && chr <= 'z' || 'A' <= chr && chr <= 'Z' ||
		'0' <= chr && chr <= '9' ||
		chr >= utf8.RuneSelf && (unicode.IsLetter(chr) || unicode.IsDigit(chr))
}
