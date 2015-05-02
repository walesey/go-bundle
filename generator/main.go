package main

import (
	log "github.com/Sirupsen/logrus"
	"io"
	"os"
)

func main() {

	gen, err := ParseAndGenerate(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(os.Stdout, gen)

}
