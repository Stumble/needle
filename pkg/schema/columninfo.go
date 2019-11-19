package schema

import (
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/types"
)

// ColumnInfo is a sql col with options.
type ColumnInfo struct {
	col *ast.ColumnDef
}

// NewColumnInfo -
func NewColumnInfo(col *ast.ColumnDef) SQLColumn {
	return &ColumnInfo{
		col: col,
	}
}

// Name - implements SQLColumn
func (c *ColumnInfo) Name() string {
	return c.col.Name.Name.String()
}

// NotNull - implements SQLColumn
func (c *ColumnInfo) NotNull() bool {
	for _, option := range c.col.Options {
		if option.Tp == ast.ColumnOptionNotNull {
			return true
		}
	}
	return false
}

// Type - SQL type.
func (c *ColumnInfo) Type() *types.FieldType {
	return c.col.Tp
}

// GoType - field in go.
func (c *ColumnInfo) GoType() GoType {
	return EvalTypeToGoType(c.col.Tp)
}

// NameExpr - of the col.
func (c *ColumnInfo) NameExpr() *ast.ColumnNameExpr {
	copy := *c.col.Name
	return &ast.ColumnNameExpr{
		Name: &copy,
	}
}
