package generator

import (
	"bytes"
	"github.com/mamaar/risotto/parser"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerator(t *testing.T) {

	testFiles, err := ioutil.ReadDir("test")
	assert.NoError(t, err)

	for _, testFile := range testFiles {
		if testFile.IsDir() ||
			strings.HasSuffix(testFile.Name(), "_output.js") ||
			strings.HasSuffix(testFile.Name(), "~") {
			continue
		}

		testName := testFile.Name()[:len(testFile.Name())-3]
		outName := "./test/" + testName + "_output.js"

		expectedFd, err := os.Open(outName)
		assert.NoError(t, err)

		in := &bytes.Buffer{}
		expected := &bytes.Buffer{}

		lookup, _ := filepath.Abs("./test/node_modules")
		popts := parser.ParserOptions{
			FileName:          filepath.Join("./test", testFile.Name()),
			ModulesLookupDirs: []string{lookup},
			ParseModular:      true,
		}
		parser, err := parser.NewParser(popts)
		assert.NoError(t, err, testName)
		module, err := parser.ParseModule()
		assert.NoError(t, err, testName)

		if parser.IsModular() {
			generated, err := GenerateModule(module)
			assert.NoError(t, err, testName)
			in.ReadFrom(generated)
		} else {
			generated, err := GenerateProgram(module.Program)
			assert.NoError(t, err, testName)
			in.ReadFrom(generated)
		}

		expected.ReadFrom(expectedFd)

		assert.Equal(t, strings.Trim(expected.String(), " \n"), strings.Trim(in.String(), " \n"), testName)
	}

}
