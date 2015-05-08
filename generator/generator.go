package generator

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gammazero/graph/toposort"
	"github.com/mamaar/risotto/ast"
	"github.com/mamaar/risotto/parser"
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
	isElseStatement    bool

	moduleCache map[string]*ast.Module
	moduleIndex map[string]int
}

// Generate builds javascript from the program
// passed as an argument.
func GenerateProgram(p *ast.Program) (io.Reader, error) {
	gen := &generator{
		buffer:      &bytes.Buffer{},
		indentation: "    ",
	}

	if err := gen.generateProgram(p); err != nil {
		return nil, err
	}

	return gen.code(), nil
}

func GenerateModule(m *ast.Module) (io.Reader, error) {
	gen := &generator{
		buffer:      &bytes.Buffer{},
		indentation: "    ",
		moduleCache: make(map[string]*ast.Module),
		moduleIndex: make(map[string]int),
	}
	if err := gen.generateModule(m); err != nil {
		return nil, err
	}

	return gen.code(), nil
}

// ParseAndGenerate takes an io.Reader to be parsed and
// generate javascript code.
func ParseAndGenerate(in io.Reader) (io.Reader, error) {
	prog, err := parser.ParseFile(nil, "<stdin>", in, parser.IgnoreRegExpErrors)
	if err != nil {
		return nil, err
	}

	return GenerateProgram(prog)
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

// Ensures that s will be the first statement on a line
func (g *generator) writeAlone(s string) {
	if g.buffer.Len() <= 0 {
		return
	}
	if g.buffer.String()[g.buffer.Len()-1] != '\n' {
		g.writeLine(s)
		return
	}
	g.writeIndentation(s)
}

func (g *generator) writeIndentation(s string) {

	if g.currentChar > 0 && g.currentChar%len(g.indentation) == 0 {
		g.write(s)
		return
	}

	inlineIndent := len(g.indentationString()) - g.currentChar%len(g.indentation)
	if inlineIndent < 0 {
		inlineIndent = 0
	}
	indent := strings.Repeat(" ", inlineIndent)
	g.write(indent + s)
	g.currentChar = inlineIndent + len(s)
}

func (g *generator) writeLine(s string) {
	g.write("\n")
	g.writeIndentation(s)
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

func (g *generator) resolveDependencies(modules *[]toposort.Edge, m *ast.Module) {
	if _, ok := g.moduleCache[m.Path]; !ok {
		*modules = append(*modules, toposort.Edge{m, nil})
		g.moduleCache[m.Path] = m
	}
	for _, dep := range m.Dependencies {
		*modules = append(*modules, toposort.Edge{m, dep})
		g.resolveDependencies(modules, dep)
	}
}

func (g *generator) generateModuleRequirementList(m *ast.Module) ([]*ast.Module, error) {
	modules := []toposort.Edge{}
	g.resolveDependencies(&modules, m)
	sorted, err := toposort.Toposort(modules, true, false)
	if err != nil {
		return nil, err
	}

	required := make(map[string]bool)
	result := []*ast.Module{}
	i := 0
	for _, m := range sorted {
		mod := m.(*ast.Module)
		if _, ok := required[mod.Path]; !ok {
			g.moduleIndex[mod.Path] = i
			result = append(result, mod)
		}
		required[mod.Path] = true
		i++
	}
	return result, nil
}

func (g *generator) generateModule(m *ast.Module) error {
	l, err := g.generateModuleRequirementList(m)
	if err != nil {
		return err
	}

	g.writeLine("{")
	g.indentLevel++

	for i, m := range l {
		g.writeIndentation(fmt.Sprintf("%d:", i))
		g.writeIndentation("[function (require, module, exports) {")
		g.indentLevel++
		g.generateProgram(m.Program)
		g.writeAlone("}, {")
		for path, dep := range m.Dependencies {
			g.write(escapeKey(path))
			g.write(":")
			g.write(fmt.Sprintf("%d", g.moduleIndex[dep.Path]))
		}
		g.indentLevel--
		g.writeAlone("}")
		g.writeAlone("}")
	}

	g.indentLevel--
	g.writeAlone("}")
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
