package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/walesey/go-bundle/parser"
)

type Loader interface {
	Load(in io.Reader) (io.Reader, error)
}

type module struct {
	name string
	data []byte
}

type _bundle struct {
	modules map[string]*module
	loaders map[string][]Loader

	moduleCounter int
}

// Bundle takes entry and loaders to load js into a single javascript bundle
func Bundle(entry string, loaders map[string][]Loader) (io.Reader, error) {
	bundle := newBundle()
	bundle.loaders = loaders

	entryModule, err := resolveModule(fmt.Sprint("./", entry), "./", bundle)
	if err != nil {
		return nil, err
	}

	// write the bundle file
	out := new(bytes.Buffer)
	out.Write([]byte("var require;"))
	out.Write([]byte("\n__go_bundle_modules__ = {};\n"))
	for _, mod := range bundle.modules {
		out.Write([]byte(fmt.Sprintf("\n__go_bundle_modules__.%v = function() {\n", mod.name)))
		out.Write([]byte("var module = { exports: {} };\n"))
		out.Write(mod.data)
		out.Write([]byte("\nreturn module.exports;\n"))
		out.Write([]byte("};\n\n"))
	}
	out.Write([]byte("require = function (module) {\n"))
	out.Write([]byte("return __go_bundle_modules__[module]();\n"))
	out.Write([]byte("};\n"))
	out.Write([]byte(fmt.Sprintf("require('%v');", entryModule)))

	return out, err
}

func resolveModule(importPath, currentPath string, bundle *_bundle) (string, error) {
	//use relative path
	if strings.HasPrefix(importPath, "./") {
		path := filepath.Join(filepath.Dir(currentPath), importPath)
		return loadModule(path, bundle)
	}

	//look in node_modules
	wd, _ := os.Getwd()
	packagePath := filepath.Join(wd, "node_modules", importPath, "package.json")
	packageData, err := ioutil.ReadFile(packagePath)
	if err != nil {
		return "", err
	}

	var pkg map[string]string
	json.Unmarshal(packageData, &pkg)
	main, ok := pkg["main"]
	if !ok {
		return "", fmt.Errorf("npm package has no main entrypoint")
	}

	path := filepath.Join(wd, "node_modules", importPath, main)
	return loadModule(path, bundle)
}

func loadModule(path string, bundle *_bundle) (string, error) {
	// check the file extention
	ext := filepath.Ext(path)
	if len(ext) == 0 {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			path = fmt.Sprint(path, ".js")
		} else {
			path = fmt.Sprint(path, "/index.js")
		}
	}

	// use the absolute path and check if the file is already loaded
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if mod, ok := bundle.modules[absPath]; ok {
		if mod.data == nil {
			return "", fmt.Errorf("circular imports not allowed: %v", absPath)
		}
		return mod.name, nil
	}

	// create a new module
	moduleName := bundle.moduleName()
	mod := &module{name: moduleName}
	bundle.modules[absPath] = mod

	// load file and transform using the loader plugins
	var src io.Reader
	src, err = os.Open(absPath)
	if err != nil {
		return moduleName, err
	}

	if loaders, ok := bundle.loaders[ext]; ok {
		for _, loader := range loaders {
			src, err = loader.Load(src)
			if err != nil {
				return moduleName, nil
			}
		}
	}

	// non js files do no not need to be parsed.
	if ext != ".js" {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, src)
		mod.data = buf.Bytes()
		return moduleName, err
	}

	// parse the js code and generate source
	prog, err := parser.ParseFile(nil, path, src, parser.IgnoreRegExpErrors&parser.StoreComments)
	if err != nil {
		return moduleName, err
	}

	gen, err := generate(prog, bundle)
	if err != nil {
		return moduleName, err
	}

	// load the generated source code into the module data
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, gen); err != nil {
		return moduleName, err
	}

	mod.data = buf.Bytes()
	return moduleName, nil
}

// moduleName - generate a unique name for a module
func (b *_bundle) moduleName() string {
	b.moduleCounter++
	return fmt.Sprint("m", b.moduleCounter)
}

func newBundle() *_bundle {
	return &_bundle{
		modules: make(map[string]*module),
		loaders: make(map[string][]Loader),
	}
}
