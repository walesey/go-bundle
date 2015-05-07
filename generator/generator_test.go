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
		if testFile.IsDir() {
			continue
		}
		if strings.HasSuffix(testFile.Name(), "_output.js") ||
			strings.HasSuffix(testFile.Name(), "~") {
			continue
		}

		testName := testFile.Name()[:len(testFile.Name())-3]
		outName := "./test/" + testName + "_output.js"

		inFd, err := os.Open("./test/" + testFile.Name())
		assert.NoError(t, err)
		expectedFd, err := os.Open(outName)
		assert.NoError(t, err)

		in := &bytes.Buffer{}
		expected := &bytes.Buffer{}

		lookup, _ := filepath.Abs("./test/node_modules")
		popts := parser.ParserOptions{
			FileName:          "<stdin>",
			ModulesLookupDirs: []string{lookup},
		}
		parser, err := parser.NewParser(inFd, popts)
		assert.NoError(t, err, testName)
		prog, err := parser.Parse()
		assert.NoError(t, err, testName)
		generated, err := Generate(prog)
		assert.NoError(t, err, testName)
		in.ReadFrom(generated)
		expected.ReadFrom(expectedFd)

		assert.Equal(t, strings.Trim(expected.String(), " \n"), strings.Trim(in.String(), " \n"), testName)
	}

}
