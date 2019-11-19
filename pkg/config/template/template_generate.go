// +build ignore

package main

import (
	"log"
	"net/http"

	"github.com/shurcooL/vfsgen"
)

func main() {
	err := vfsgen.Generate(
		http.Dir("./templates"),
		vfsgen.Options{
			Filename:     "./template_vfsdata.go",
			PackageName:  "template",
			VariableName: "templateAssets",
		})
	if err != nil {
		log.Fatalln(err)
	}
}
