package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/walesey/go-bundle/cssLoader"
	"github.com/walesey/go-bundle/generator"
)

func main() {
	entry := "./index.js"
	if len(os.Args) >= 2 {
		entry = os.Args[1]
	}

	styleLoader := cssLoader.New(cssLoader.Config{
		ClassNaming: "[name]-[hash]",
		Outfile:     "./styles.css",
	})

	loaders := map[string][]generator.Loader{
		".css": []generator.Loader{styleLoader},
	}

	gen, err := generator.Bundle(entry, loaders)
	if err != nil {
		fmt.Println(err)
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(gen)

	fmt.Print(string(buf.Bytes()))
}
