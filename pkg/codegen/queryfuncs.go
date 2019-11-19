package codegen

import (
	"bytes"
	"text/template"
	"time"

	codetemplates "github.com/stumble/needle/pkg/codegen/template"
)

var queryfuncTemplate *template.Template

func init() {
	var err error
	queryfuncTemplate, err = template.New("QueryFunc").Parse(codetemplates.GetQueryFuncTemplate())
	if err != nil {
		panic(err)
	}
}

// QueryFuncTemplate - the query function.
type QueryFuncTemplate struct {
	RepoName        string
	QueryName       string
	QueryInnerSig   string
	QuerySig        string
	HiddenQueryName string
	RstTypeName     string
	CacheDuration   *time.Duration // nil = nocache, 0s = forever.
	SQLVarName      string
	IsList          bool
	InitArgsType    string // init a empty args in the ourter func, if not empty string.
}

// Generate string template
func (q QueryFuncTemplate) Generate() (string, error) {
	buf := bytes.NewBufferString("")
	err := queryfuncTemplate.Execute(buf, q)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
