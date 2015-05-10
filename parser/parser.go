package parser

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"

	"github.com/mamaar/risotto/ast"
	"github.com/mamaar/risotto/file"
	"github.com/mamaar/risotto/token"
	"os"
	"path/filepath"
	"strings"
)

// A Mode value is a set of flags (or 0). They control optional parser functionality.
type Mode uint

const (
	IgnoreRegExpErrors Mode = 1 << iota // Ignore RegExp compatibility errors (allow backtracking)
)

type _parser struct {
	filename string
	filepath string
	str      string
	length   int
	base     int

	chr       rune // The current character
	chrOffset int  // The offset of current character
	offset    int  // The offset after current character (may be greater than 1)

	idx     file.Idx    // The index of token
	token   token.Token // The token
	literal string      // The literal of the token, if any

	scope             *_scope
	insertSemicolon   bool // If we see a newline, then insert an implicit semicolon
	implicitSemicolon bool // An implicit semicolon exists

	errors ErrorList

	recover struct {
		// Scratch when trying to seek to the next statement, etc.
		idx   file.Idx
		count int
	}

	mode Mode
	file *file.File

	parserOptions     ParserOptions
	parseModular      bool
	isModular         bool
	modulesLookupDirs []string
	rootModule        *ast.Module
}

// ParserOptions holds options passed to the parser on initialization
// currently only used by the function NewParser.
type ParserOptions struct {
	FileName string

	ParseModular      bool
	ModulesLookupDirs []string
	Modules           map[string]*ast.Program
}

func _newParser(filename, src string, base int) *_parser {
	return &_parser{
		str:    src,
		offset: -1,
		length: len(src),
		base:   base,
		file:   file.NewFile(filename, src, base),
	}
}

func newParser(filename, src string) *_parser {
	return _newParser(filename, src, 1)
}

// NewParser creates a parser object using custom options
func NewParser(options ParserOptions) (*_parser, error) {
	filePath, err := filepath.Abs(options.FileName)
	fileName := filepath.Base(filePath)
	if err != nil {
		return nil, err
	}
	if options.FileName == "<stdin>" {
		fileName = "<stdin>"
		filePath, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	lookupDirs := []string{}
	for _, lookupDir := range options.ModulesLookupDirs {
		if stat, _ := os.Stat(lookupDir); stat == nil {
			continue
		}
		lookupDirs = append(lookupDirs, lookupDir)
	}

	fd, err := os.Open(filePath)
	if err != nil {
		fd.Close()
		return nil, err
	}
	defer fd.Close()

	buf := bytes.Buffer{}
	buf.ReadFrom(fd)

	return &_parser{
		filename: fileName,
		filepath: filePath,
		str:      buf.String(),
		offset:   -1,
		length:   buf.Len(),
		base:     1,
		file:     file.NewFile(fileName, buf.String(), 1),

		parserOptions:     options,
		parseModular:      options.ParseModular,
		modulesLookupDirs: lookupDirs,
		rootModule: &ast.Module{
			Path:         filePath,
			Dependencies: make(map[string]*ast.Module),
		},
	}, nil
}

func ReadSource(filename string, src interface{}) ([]byte, error) {
	if src != nil {
		switch src := src.(type) {
		case string:
			return []byte(src), nil
		case []byte:
			return src, nil
		case *bytes.Buffer:
			if src != nil {
				return src.Bytes(), nil
			}
		case io.Reader:
			var bfr bytes.Buffer
			if _, err := io.Copy(&bfr, src); err != nil {
				return nil, err
			}
			return bfr.Bytes(), nil
		}
		return nil, errors.New("invalid source")
	}
	return ioutil.ReadFile(filename)
}

// ParseFile parses the source code of a single JavaScript/ECMAScript source file and returns
// the corresponding ast.Program node.
//
// If fileSet == nil, ParseFile parses source without a FileSet.
// If fileSet != nil, ParseFile first adds filename and src to fileSet.
//
// The filename argument is optional and is used for labelling errors, etc.
//
// src may be a string, a byte slice, a bytes.Buffer, or an io.Reader, but it MUST always be in UTF-8.
//
//      // Parse some JavaScript, yielding a *ast.Program and/or an ErrorList
//      program, err := parser.ParseFile(nil, "", `if (abc > 1) {}`, 0)
//
func ParseFile(fileSet *file.FileSet, filename string, src interface{}, mode Mode) (*ast.Program, error) {
	str, err := ReadSource(filename, src)
	if err != nil {
		return nil, err
	}
	{
		str := string(str)

		base := 1
		if fileSet != nil {
			base = fileSet.AddFile(filename, str)
		}

		parser := _newParser(filename, str, base)
		parser.mode = mode
		return parser.parse()
	}
}

func (self *_parser) slice(idx0, idx1 file.Idx) string {
	from := int(idx0) - self.base
	to := int(idx1) - self.base
	if from >= 0 && to <= len(self.str) {
		return self.str[from:to]
	}

	return ""
}

func (self *_parser) Parse() (*ast.Program, error) {
	return self.parse()
}

func (self *_parser) parse() (*ast.Program, error) {
	self.next()
	return self.parseProgram()
}

// rawNext moves pointer to the next token
func (self *_parser) rawNext() {
	self.token, self.literal, self.idx = self.scan()
}

// next moves pointer to next non-whitespace token
func (self *_parser) next() {
	for {
		self.token, self.literal, self.idx = self.scan()
		if self.token != token.WHITESPACE {
			break
		}
	}
}

func (self *_parser) optionalSemicolon() {
}

func (self *_parser) semicolon() error {
	if self.token != token.RIGHT_PARENTHESIS && self.token != token.RIGHT_BRACE {
		if self.implicitSemicolon {
			self.implicitSemicolon = false
			return nil
		}

		if _, err := self.expect(token.SEMICOLON); err != nil {
			return err
		}
	}
	return nil
}

func (self *_parser) idxOf(offset int) file.Idx {
	return file.Idx(self.base + offset)
}

func (self *_parser) expect(value token.Token) (file.Idx, error) {
	idx := self.idx
	if self.token != value {
		if err := self.errorUnexpectedToken(self.token); err != nil {
			return file.Idx(-1), err
		}
	}
	self.next()
	return idx, nil
}

func lineCount(str string) (int, int) {
	line, last := 0, -1
	pair := false
	for index, chr := range str {
		switch chr {
		case '\r':
			line += 1
			last = index
			pair = true
			continue
		case '\n':
			if !pair {
				line += 1
			}
			last = index
		case '\u2028', '\u2029':
			line += 1
			last = index + 2
		}
		pair = false
	}
	return line, last
}

func (self *_parser) position(idx file.Idx) file.Position {
	position := file.Position{}
	offset := int(idx) - self.base
	str := self.str[:offset]
	position.Filename = self.filename
	line, last := lineCount(str)
	position.Line = 1 + line
	if last >= 0 {
		position.Column = offset - last
	} else {
		position.Column = 1 + len(str)
	}

	return position
}

// isRequireModule determines whether the callee is 'require' and returns the module path
func (self *_parser) isRequireModule(c ast.Expression, argumentList []ast.Expression) (string, bool) {
	callee, ok := c.(*ast.Identifier)
	if !ok {
		return "", false
	}
	if callee.Name != "require" {
		return "", false
	}
	if len(argumentList) == 0 {
		return "", false
	}
	module, ok := argumentList[0].(*ast.StringLiteral)
	if !ok {
		return "", false
	}

	return module.Value, true
}

func (self *_parser) resolveRelativePath(path string) (string, bool) {
	abs := filepath.Join(filepath.Dir(self.filepath), path)
	if len(filepath.Ext(abs)) == 0 {
		if s, _ := os.Stat(abs); s != nil && s.IsDir() {
			indexPath := filepath.Join(abs, "index.js")
			namePath := filepath.Join(abs, filepath.Base(path)+".js")
			indexStat, _ := os.Stat(indexPath)
			nameStat, _ := os.Stat(namePath)

			if indexStat != nil {
				return indexPath, true
			}
			if nameStat != nil {
				return namePath, true
			}

		}
	}
	if filepath.Ext(abs) != ".js" {
		abs += ".js"
	}

	fd, err := os.Open(abs)
	if err != nil {
		return path, false
	}
	fd.Close()
	return abs, true
}

// resolvePath resolves a module path based on if it's relative or global.
func (self *_parser) resolvePath(path string) (string, bool) {
	if strings.HasPrefix(path, ".") {
		return self.resolveRelativePath(path)
	}

	for _, d := range self.modulesLookupDirs {
		abs, _ := filepath.Abs(filepath.Join(d, path))
		fd, err := os.Open(abs)
		if err != nil {
			fd.Close()
			continue
		}
		defer fd.Close()
		if fInfo, err := fd.Stat(); err == nil && fInfo.IsDir() {
			indexPath := filepath.Join(abs, "index.js")
			namePath := filepath.Join(abs, path+".js")

			indexStat, _ := os.Stat(indexPath)
			nameStat, _ := os.Stat(namePath)
			if indexStat != nil {
				return indexPath, true
			}
			if nameStat != nil {
				return namePath, true
			}
			continue
		}

		return abs, true
	}
	return path, false
}

// Open the entrypoint for a module for reading
func (self *_parser) ParseModule() (*ast.Module, error) {
	if err := self.parseModule(self.filepath); err != nil {
		return nil, err
	}
	return self.rootModule, nil
}

func (self *_parser) parseModule(path string) error {
	self.rootModule = &ast.Module{
		Path:         path,
		Dependencies: make(map[string]*ast.Module),
	}
	self.next()
	prog, err := self.parseProgram()
	if err != nil {
		return err
	}
	self.rootModule.Program = prog

	return nil
}

func (self *_parser) IsModular() bool {
	return self.isModular
}
