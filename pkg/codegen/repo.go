package codegen

import (
	"bytes"
	"text/template"

	codetemplates "github.com/stumble/needle/pkg/codegen/template"
)

var repoTemplate *template.Template

func init() {
	var err error
	repoTemplate, err = template.New("Repo").Parse(codetemplates.GetRepoTemplate())
	if err != nil {
		panic(err)
	}
}

// SQL statements
type SQLStatementDecl struct {
	VarName    string
	EscapedSQL string
}

// RepoTemplate template for render a repo.
type RepoTemplate struct {
	NeedleVersion       string
	TableSchema         string
	PkgName             string
	InterfaceName       string
	InterfaceSignatures []string
	RepoName            string
	MainStruct          string
	MainStructName      string
	LoadDump            string
	Statements          []SQLStatementDecl
	Queries             []string
	Mutations           []string
}

// Generate string template
func (q RepoTemplate) Generate() (string, error) {
	buf := bytes.NewBufferString("")
	err := repoTemplate.Execute(buf, q)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
