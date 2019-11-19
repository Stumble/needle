package visitors

import (
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/model"

	"github.com/stumble/needle/pkg/schema"
	"github.com/stumble/needle/pkg/utils"
)

// NameResolveVisitor -
type NameResolveVisitor struct {
	*baseVisitor
	db []schema.SQLTable
}

// NewNameResolveVisitor -
// after: all ColumnName will have fully qualified names (i.e. table.col), aliasing will be kept.
func NewNameResolveVisitor(db []schema.SQLTable) *NameResolveVisitor {
	return &NameResolveVisitor{
		baseVisitor: newBaseVisitor("NameResolve"),
		db:          db,
	}
}

var _ ast.Visitor = &NameResolveVisitor{}

// return true if we found the column in the table
func (c *NameResolveVisitor) lookup(colname string, tablename string) bool {
	for _, table := range c.db {
		if table.Name() == tablename {
			for _, col := range table.Columns() {
				if col.Name() == colname {
					return true
				}
			}
			return false
		}
	}
	return false
}

func (c *NameResolveVisitor) resolve(col string, ref *ast.TableRefsClause) (string, bool) {
	if ref == nil {
		return "", false
	}
	join := ref.TableRefs
	left := join.Left
	right := join.Right
	var rst []string // table candidates.

	for _, node := range []ast.ResultSetNode{left, right} {
		if node == nil {
			continue
		}
		switch v := node.(type) {
		case *ast.SelectStmt, *ast.SubqueryExpr:
			c.LogCE("Subquery are not supported: %s", utils.RestoreNode(v))
			c.AppendErr(NewError(ErrNotSupported, "subquery"))
		case *ast.TableSource:
			name, simple := v.Source.(*ast.TableName)
			if !simple {
				// not a simple table, not supported for now
				c.AppendErr(NewError(ErrNotSupported, v.Text()))
				return "", false
			}
			tablename := name.Name.String()
			if c.lookup(col, tablename) {
				rst = append(rst, tablename)
			}
		case *ast.TableName:
			tablename := v.Name.String()
			if c.lookup(col, tablename) {
				rst = append(rst, tablename)
			}
		case *ast.Join:
			name, found := c.resolve(col, &ast.TableRefsClause{TableRefs: v})
			if found {
				rst = append(rst, name)
			}
		}
	}
	// ambiguous col names not allowed.
	if len(rst) == 1 {
		return rst[0], true
	}
	c.AppendErr(NewErrorf(ErrInvalidExpr, "ambiguous expression: %s, multiple defs: %v", col, rst))
	return "", false
}

// return a string of the closest type.
func (c *NameResolveVisitor) findClosestDef(col string) (string, bool) {
	node, ok := c.FindInCtxAnyOf(
		(*ast.SelectStmt)(nil), (*ast.UpdateStmt)(nil),
		(*ast.DeleteStmt)(nil), (*ast.InsertStmt)(nil))
	if ok {
		var from *ast.TableRefsClause
		switch tn := node.(type) {
		case *ast.SelectStmt:
			from = tn.From
		case *ast.UpdateStmt:
			from = tn.TableRefs
		case *ast.DeleteStmt:
			from = tn.TableRefs
		case *ast.InsertStmt:
			from = tn.Table
		default:
			c.AppendErr(NewError(ErrCompilerError, "unexpected find ctx anyof return type"))
			return "", false
		}
		nm, found := c.resolve(col, from)
		if !found {
			c.AppendErr(NewErrorf(ErrInvalidExpr, "cannot find the column of (%s)", col))
			return "", false
		}
		return nm, true
	}
	return "", false
}

// Enter - Implements Visitor
func (c *NameResolveVisitor) Enter(n ast.Node) (ast.Node, bool) {
	c.baseVisitor.Enter(n)
	switch v := n.(type) {
	case *ast.ColumnName:
		if v.Schema.String() != "" {
			c.AppendErr(NewErrorf(ErrCompilerError, "schema not supported: %v", v))
		}
		tableName := v.Table.String()
		// unqualified name resovled to closed select/update/delete expr.
		if tableName == "" {
			nm, ok := c.findClosestDef(v.Name.String())
			if ok {
				tableName = nm
			}
		}
		v.Table = model.NewCIStr(tableName)
	}
	return n, false
}

// Leave - Implements Visitor
func (c *NameResolveVisitor) Leave(n ast.Node) (ast.Node, bool) {
	c.baseVisitor.Leave(n)
	return n, true
}
