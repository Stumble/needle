package codegen

import (
	"bytes"
	"text/template"

	codetemplates "github.com/stumble/needle/pkg/codegen/template"
)

var loaddumpfuncTemplate *template.Template

func init() {
	var err error
	loaddumpfuncTemplate, err =
		template.New("LoadDumpFunc").Parse(codetemplates.GetLoadDumpTemplate())
	if err != nil {
		panic(err)
	}
}

// LoadDumpFuncTemplate - the load and dump functions.
type LoadDumpFuncTemplate struct {
	RepoName       string
	MainStructName string
	SelectAllSQL   string
	InsertRowSQL   string
}

// Generate string template
func (l LoadDumpFuncTemplate) Generate() (string, error) {
	buf := bytes.NewBufferString("")
	err := loaddumpfuncTemplate.Execute(buf, l)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
