package visitors

import (
	"github.com/pingcap/parser/ast"

	"github.com/stumble/needle/pkg/schema"
	"github.com/stumble/needle/pkg/utils"
)

// StarElimVisitor - eliminate * in select by replacing it with a list of fields.
type StarElimVisitor struct {
	*baseVisitor
	table schema.SQLTable
}

// NewStarElimVisitor -
func NewStarElimVisitor(tb schema.SQLTable) *StarElimVisitor {
	return &StarElimVisitor{
		baseVisitor: newBaseVisitor("StarElim"),
		table:       tb,
	}
}

var _ ast.Visitor = &StarElimVisitor{}

// Enter - Implements Visitor
func (s *StarElimVisitor) Enter(n ast.Node) (ast.Node, bool) {
	s.baseVisitor.Enter(n)
	switch v := n.(type) {
	case *ast.SelectStmt:
		fields := v.Fields.Fields
		if hasWildcard(fields) {
			if len(fields) == 1 {
				v.Fields = s.makeTableFields()
			} else {
				s.AppendErr(NewErrorf(ErrInvalidExpr,
					"* with extra fields are not allowed: %s", utils.RestoreNode(n)))
			}
		}
	}
	return n, false
}

// Leave - Implements Visitor
func (s *StarElimVisitor) Leave(n ast.Node) (ast.Node, bool) {
	s.baseVisitor.Leave(n)
	return n, true
}

func (s *StarElimVisitor) makeTableFields() *ast.FieldList {
	rst := make([]*ast.SelectField, 0)
	for _, col := range s.table.StarColumns() {
		selectField := &ast.SelectField{
			Expr: col.NameExpr(),
		}
		rst = append(rst, selectField)
	}
	return &ast.FieldList{
		Fields: rst,
	}
}

func hasWildcard(fields []*ast.SelectField) bool {
	for _, f := range fields {
		if f.WildCard != nil {
			return true
		}
	}
	return false
}
