package cssLoader

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/aymerick/douceur/css"
	"github.com/aymerick/douceur/parser"
)

//TODO: Implement the correct Class nameing and Outfile configs
type Config struct {
	ClassNaming string
	Outfile     string
}

type CssLoader struct {
	config  Config
	buffer  *bytes.Buffer
	out     io.Writer
	classes map[string]string
	hash    string
}

func New(config Config) *CssLoader {
	return &CssLoader{config: config}
}

func (l *CssLoader) Load(src io.Reader) (io.Reader, error) {
	srcData, err := ioutil.ReadAll(src)
	if err != nil {
		return nil, err
	}

	l.buffer = new(bytes.Buffer)
	l.classes = make(map[string]string)
	l.hash = fmt.Sprintf("%x", md5.Sum(srcData))

	styles, err := parser.Parse(string(srcData))
	if err != nil {
		return nil, err
	}

	out, err := os.Create(l.config.Outfile)
	if err != nil {
		return nil, err
	}

	defer out.Close()
	l.out = out

	l.parseStyles(styles)

	return l.buffer, nil
}

func (l *CssLoader) parseStyles(styles *css.Stylesheet) {
	for _, rule := range styles.Rules {

		//parse selectors
		for i, sel := range rule.Selectors {
			for _, s := range strings.Split(sel, " ") {
				class := l.parseClass(s)
				sel = strings.Replace(sel, s, class, 1)
			}
			l.write(sel)
			if i != len(rule.Selectors)-1 {
				l.write(", ")
			}
		}

		//parse declarations
		l.write(" {")
		for _, dec := range rule.Declarations {
			l.writeIndent(dec.String())
		}
		l.write("\n}\n")
	}
}

func (l *CssLoader) parseClass(selector string) string {
	if !strings.HasPrefix(selector, ".") {
		return selector
	}

	parts := strings.Split(selector, ":")
	class := parts[0]
	className := fmt.Sprint(l.hash[:6], "_", strings.TrimPrefix(class, "."))
	if _, ok := l.classes[className]; !ok {
		l.writeModule(fmt.Sprintf("module.exports%v = '%v';\n", class, className))
		l.classes[className] = class
	}

	result := fmt.Sprint(".", className)
	if len(parts) >= 2 {
		result = fmt.Sprint(result, ":", parts[1])
	}
	return result
}

func (l *CssLoader) writeModule(s string) {
	l.buffer.Write([]byte(s))
}

func (l *CssLoader) write(s string) {
	l.out.Write([]byte(s))
}

func (l *CssLoader) writeIndent(s string) {
	l.out.Write([]byte("\n  "))
	l.out.Write([]byte(s))
}
