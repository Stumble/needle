package visitors

import (
	"errors"

	"github.com/pingcap/tidb/parser/ast"

	"github.com/stumble/needle/pkg/schema"
	"github.com/stumble/needle/pkg/utils"
)

// GoVar - is a go variable with name and type.
type GoVar struct {
	TableName string
	Name      string
	Type      schema.GoType
}

// OutputExtractVisitor -
type OutputExtractVisitor struct {
	*baseVisitor

	Output []GoVar
}

// NewOutputExtractVisitor -
func NewOutputExtractVisitor() *OutputExtractVisitor {
	return &OutputExtractVisitor{
		baseVisitor: newBaseVisitor("OutputExtract"),
	}
}

var _ ast.Visitor = &OutputExtractVisitor{}

// Enter - Implements Visitor
func (s *OutputExtractVisitor) Enter(n ast.Node) (ast.Node, bool) {
	s.baseVisitor.Enter(n)
	selectStmt, ok := n.(*ast.SelectStmt)
	if !ok {
		s.AppendErr(NewErrorf(ErrCompilerError,
			"computing output of statement that is not allowed: %s", utils.RestoreNode(n)))
		return n, true
	}

	for _, f := range selectStmt.Fields.Fields {
		if f.WildCard != nil {
			panic("wildcard is not eliminated, midend skipped?")
		}
		vv, err := calcGoVar(f)
		if err != nil {
			s.AppendErr(NewErrorf(ErrInvalidExpr, "failed to construct govar name: %s",
				utils.RestoreNode(n)))
			return n, true
		}
		s.Output = append(s.Output, *vv)
	}

	// do not visit children
	return n, true
}

// Leave - Implements Visitor
func (s *OutputExtractVisitor) Leave(n ast.Node) (ast.Node, bool) {
	s.baseVisitor.Leave(n)
	return n, true
}

// columnNameExpr: (Table, ColName)
// function:       ("", AsName)
func calcGoVar(field *ast.SelectField) (*GoVar, error) {
	if field.AsName.String() != "" {
		return &GoVar{
			TableName: "",
			Name:      field.AsName.String(),
			Type:      schema.EvalTypeToGoType(field.Expr.GetType()),
		}, nil
	}
	switch v := field.Expr.(type) {
	case *ast.ColumnNameExpr:
		return &GoVar{
			TableName: v.Name.Table.String(),
			Name:      v.Name.Name.String(),
			Type:      schema.EvalTypeToGoType(v.GetType()),
		}, nil
	default:
		return nil, errors.New("failed to calc GoVar name")
	}
}
