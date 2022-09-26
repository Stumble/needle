package codegen

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/iancoleman/strcase"
)

var gostructTemplate *template.Template

func init() {
	var err error
	gostructTemplate, err = template.New("GoStruct").Parse(`
// {{.Name}} - {{.Comments}}
type {{.Name}} struct {
{{range .Fields}}{{.}}
{{end}}
}
`)
	if err != nil {
		panic(err)
	}
}

// GoType - a simple type in go.
// If both pointer and list, it represents []*T, not *[]T, i.e. nullability is on the inner
// type.
type GoType struct {
	Pkg       string
	ID        string
	IsPointer bool
	IsList    bool
}

func (g GoType) String() string {
	rst := g.ID
	if g.Pkg != "" {
		i := strings.LastIndex(g.Pkg, "/")
		pkgname := g.Pkg[i+1:]
		rst = pkgname + "." + rst
	}
	if g.IsPointer {
		rst = "*" + rst
	}
	if g.IsList {
		rst = "[]" + rst
	}
	return rst
}

// GoField - a struct field.
type GoField struct {
	Name string
	Type GoType
	Tags string
}

func NewGoField(nm string, t GoType, tags string) GoField {
	return GoField{
		Name: strcase.ToCamel(nm),
		Type: t,
		Tags: tags,
	}
}

func (g GoField) String() string {
	rst := fmt.Sprintf(`%s %s`, g.Name, g.Type)
	if g.Tags != "" {
		rst += fmt.Sprintf(` "%s"`, g.Tags)
	}
	return rst
}

// GoStruct - a go struct.
type GoStruct struct {
	Name     string
	Fields   []GoField
	Comments string
}

func (g GoStruct) String() string {
	buf := bytes.NewBufferString("")
	err := gostructTemplate.Execute(buf, g)
	if err != nil {
		panic(err)
	}
	return buf.String()
}

func (g GoStruct) IsEmpty() bool {
	return len(g.Fields) == 0
}

// ScanFunc - return a simple func string of sql scan.
func (g GoStruct) ScanFunc() string {
	fieldnames := make([]string, 0)
	for _, f := range g.Fields {
		fieldnames = append(fieldnames, "&r."+f.Name)
	}
	fieldstr := strings.Join(fieldnames, ",\n")
	return fmt.Sprintf(
		"func (r *%s) scan(sc rowScanner) error {\nreturn sc.Scan(\n%s)\n}\n", g.Name, fieldstr)
}

// KeyFunc - return a key func of go struct.
func (g GoStruct) KeyFunc(prefix string) string {
	key := ""
	if len(g.Fields) == 0 {
		key = prefix
	} else {
		var builder strings.Builder
		builder.WriteString(prefix)
		builder.WriteString(strings.Repeat(":%s", len(g.Fields)))
		key = builder.String()
	}
	fieldnames := make([]string, 0)
	for _, f := range g.Fields {
		fieldnames = append(fieldnames, "valueToString(r."+f.Name+")")
	}
	fieldstr := strings.Join(fieldnames, ",\n")
	return fmt.Sprintf(
		"// Key - cache key\n"+
			"func (r *%s) Key() string {\nreturn fmt.Sprintf(\"%s\", %s)\n}\n",
		g.Name, key, fieldstr)
}

// ArglistFunc - return a function that generates an argument list.
func (g GoStruct) ArglistFunc() string {
	var builder strings.Builder
	for _, f := range g.Fields {
		if !f.Type.IsList {
			builder.WriteString(fmt.Sprintf("args = append(args, %s)\n", "r."+f.Name))
		} else {
			tpl := `for _, v := range %s {
	args = append(args, v)
}
`
			builder.WriteString(fmt.Sprintf(tpl, "r."+f.Name))
			builder.WriteString(fmt.Sprintf("inlens = append(inlens, len(%s))\n", "r."+f.Name))
		}
	}
	return fmt.Sprintf(
		"func (r *%s) arglist() (args []interface{}, inlens []int) {\n %s return\n}\n",
		g.Name, builder.String())
}

// QueryFunc - a query function
type QueryFunc struct {
	Name          string
	SQL           string
	CacheDuration *time.Duration
	Input         *GoStruct
	Output        *GoStruct
	IsList        bool
}

// ReturnType of the query func
func (q QueryFunc) ReturnType() string {
	nm := q.Output.Name
	if q.IsList {
		return "[]" + nm
	}
	return "*" + nm
}

// Signature returns the type signature of the query function, exposed to user.
func (q QueryFunc) Signature() string {
	args := fmt.Sprintf(", args *%s", q.Input.Name)
	if q.Input.IsEmpty() {
		args = ""
	}
	return fmt.Sprintf(
		"(ctx context.Context %s, options ...Option) (%s, error)",
		args, q.ReturnType())
}

// SignatureInnerFunc returns the type signature of the inner query function.
func (q QueryFunc) SignatureInnerFunc() string {
	return fmt.Sprintf(
		"(ctx context.Context, exec DBExecuter, args *%s) (%s, error)",
		q.Input.Name, q.ReturnType())
}

// MutationFunc - a query function
type MutationFunc struct {
	Name        string
	SQL         string
	Input       *GoStruct
	Invalidates []*QueryFunc
}

// Signature returns the type signature of the mutation, exposed to user. Invalidates
// are explicitly named in signatures.
func (m MutationFunc) Signature() string {
	var invalidates strings.Builder
	for i, v := range m.Invalidates {
		invalidates.WriteString(
			fmt.Sprintf(", key%d *%s, val%d %s", i, v.Input.Name, i, v.ReturnType()))
	}
	return fmt.Sprintf(
		"(ctx context.Context, args *%s %s, options ...Option) (sql.Result, error)",
		m.Input.Name, invalidates.String(),
	)
}
