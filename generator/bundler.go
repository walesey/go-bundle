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

const globalJS = `
var require;
var process = { env: {} };
var __go_bundle_modules__ = {};
var __go_bundle_module_cache__ = {};
`

const requireJS = `
require = function (module) {
  var result = __go_bundle_module_cache__[module];
  if (!result) {
    result = __go_bundle_modules__[module]();
    __go_bundle_module_cache__[module] = result;
  }
  return result;
};
`

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

	entryModule, err := bundle.resolveModule(fmt.Sprint("./", entry), "./")
	if err != nil {
		return nil, err
	}

	// write the bundle file
	out := new(bytes.Buffer)
	out.Write([]byte(globalJS))
	for path, mod := range bundle.modules {
		out.Write([]byte(fmt.Sprint("\n// ", path)))
		out.Write([]byte(fmt.Sprintf("\n__go_bundle_modules__.%v = function() {\n", mod.name)))
		out.Write([]byte("var exports = {};\n"))
		out.Write([]byte("var module = { exports: exports };\n"))
		out.Write(mod.data)
		out.Write([]byte("\nreturn module.exports;\n"))
		out.Write([]byte("};\n\n"))
	}
	out.Write([]byte(requireJS))
	out.Write([]byte(fmt.Sprintf("require('%v');", entryModule)))

	return out, err
}

func (bundle *_bundle) resolveModule(importValue, currentPath string) (string, error) {
	//use relative path
	if strings.HasPrefix(importValue, ".") {
		path := filepath.Join(filepath.Dir(currentPath), importValue)
		return bundle.loadModule(path)
	}

	//look in node_modules
	searchPath, err := filepath.Abs(currentPath)
	if err != nil {
		return "", err
	}

	// search for node_modules in all subdirectories
	for i := 0; i < 10; i++ {
		nodeModulesPath, err := getNodeModulePath(searchPath)
		if err != nil {
			return "", err
		}

		path := filepath.Join(nodeModulesPath, importValue)
		ext := filepath.Ext(path)
		if len(ext) > 0 {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				return bundle.loadModule(path)
			}
		} else {
			packagePath := filepath.Join(path, "package.json")
			if _, err := os.Stat(packagePath); !os.IsNotExist(err) {
				packageData, err := ioutil.ReadFile(packagePath)
				if err != nil {
					return "", err
				}

				var pkg map[string]interface{}
				err = json.Unmarshal(packageData, &pkg)
				if err != nil {
					return "", err
				}

				mainPath := "index.js"
				if main, ok := pkg["main"]; ok {
					mainPath = main.(string)
				}

				return bundle.loadModule(filepath.Join(path, mainPath))
			}

			if _, err := os.Stat(fmt.Sprint(path, ".js")); !os.IsNotExist(err) {
				return bundle.loadModule(fmt.Sprint(path, ".js"))
			}

			if _, err := os.Stat(fmt.Sprint(path, ".json")); !os.IsNotExist(err) {
				return bundle.loadModule(fmt.Sprint(path, ".json"))
			}

			if _, err := os.Stat(fmt.Sprint(path, "/index.js")); !os.IsNotExist(err) {
				return bundle.loadModule(fmt.Sprint(path, "/index.js"))
			}
		}
		searchPath = filepath.Join(nodeModulesPath, "../..")
	}
	return "", fmt.Errorf("Failed to resolve Module")
}

func (bundle *_bundle) loadModule(path string) (string, error) {
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
	if filepath.Ext(absPath) != ".js" {
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

	gen, err := generate(prog, path, bundle)
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

func getNodeModulePath(path string) (string, error) {
	for i := 0; i < 10; i++ {
		files, err := filepath.Glob(filepath.Join(path, "node_modules"))
		if err != nil {
			return "", err
		}
		if files != nil && len(files) > 0 {
			return files[0], nil
		}
		path = filepath.Join(path, "..")
	}
	return "", fmt.Errorf("No node_modules directory can be found in any subdirectory")
}

func newBundle() *_bundle {
	return &_bundle{
		modules: make(map[string]*module),
		loaders: make(map[string][]Loader),
	}
}
