package codegen

import (
	"bytes"
	"text/template"
	"time"

	codetemplates "github.com/stumble/needle/pkg/codegen/template"
)

var mutationTemplate *template.Template

func init() {
	var err error
	mutationTemplate, err = template.New("MutationFunc").Parse(
		codetemplates.GetMutationFuncTemplate())
	if err != nil {
		panic(err)
	}
}

// InvalidateTemplate - the invalidate piece in function
type InvalidateTemplate struct {
	ArgName       string
	ValName       string
	CacheDuration time.Duration
}

// MutationFuncTemplate - the mutation function.
type MutationFuncTemplate struct {
	RepoName     string
	MutationName string
	MutationSig  string
	SQLVarName   string
	Invalidates  []InvalidateTemplate
}

// Generate string template
func (q MutationFuncTemplate) Generate() (string, error) {
	buf := bytes.NewBufferString("")
	err := mutationTemplate.Execute(buf, q)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
