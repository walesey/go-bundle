package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/mamaar/risotto/generator"
)

func main() {
	entry := "./index.js"
	if len(os.Args) >= 2 {
		entry = os.Args[1]
	}

	// b.AddLoaders(".css", cssLoader.New(cssLoader.Config{
	// 	ClassNaming: "[name]-[hash]",
	// 	Outfile:     "./styles.css",
	// }))

	fd, err := os.Open(entry)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer fd.Close()

	gen, err := generator.ParseAndGenerate(fd)
	if err != nil {
		fmt.Println(err)
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(gen)

	fmt.Print(string(buf.Bytes()))
}
