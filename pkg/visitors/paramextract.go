package visitors

import (
	"fmt"
	"sort"

	"github.com/pingcap/parser/ast"
	driver "github.com/pingcap/tidb/types/parser_driver"

	"github.com/stumble/needle/pkg/schema"
	"github.com/stumble/needle/pkg/utils"
)

const (
	// LimitOffset is param name of the first ? of limit ?,?
	LimitOffset = "Offset"
	// LimitCount is param name of the second ? of limit ?,?
	LimitCount = "Count"
)

// GoParam is a param with name, order, and the pointer to param obj.
type GoParam struct {
	Name      string
	TableName string
	InPattern bool
	Order     int
	Marker    *driver.ParamMarkerExpr
}

// GoType return GoType of the param.
func (g GoParam) GoType() schema.GoType {
	return schema.EvalTypeToGoType(g.Marker.GetType())
}

func (g GoParam) String() string {
	in := ""
	if g.InPattern {
		in = "[]"
	}
	return fmt.Sprintf("{$%d, %s.%s%s: %s}", g.Order, g.TableName, g.Name, in, g.GoType())
}

// ParamExtractVisitor - only mysql driver are supported, backend pass.
// premis: insert stmt has only one value(?, ?...) markers.
type ParamExtractVisitor struct {
	*baseVisitor

	Params []GoParam
}

// NewParamExtractVisitor -
func NewParamExtractVisitor() *ParamExtractVisitor {
	return &ParamExtractVisitor{
		baseVisitor: newBaseVisitor("ParamExtract"),
	}
}

var _ ast.Visitor = &ParamExtractVisitor{}

// returns name fqname succ
func (c *ParamExtractVisitor) findNameInContext(n ast.Node) (string, string, bool) {
	op, ok := c.FindInCtxAnyOf((*ast.Limit)(nil), (*ast.BinaryOperationExpr)(nil),
		(*ast.PatternInExpr)(nil), (*ast.InsertStmt)(nil), (*ast.Assignment)(nil))
	if !ok {
		return "", "", false
	}

	switch v := op.(type) {
	case *ast.Limit:
		if n == v.Count {
			return LimitCount, "", true
		} else if n == v.Offset {
			return LimitOffset, "", true
		} else {
			return "", "", false
		}
	case *ast.PatternInExpr:
		expr, isRef := v.Expr.(*ast.ColumnNameExpr)
		if !isRef {
			c.AppendErr(NewErrorf(ErrNotSupported,
				"InPattern with noncolumn expr: %s", utils.RestoreNode(v)))
			return "", "", false
		}
		return expr.Name.Name.String(), expr.Name.Table.String(), true
	case *ast.BinaryOperationExpr:
		left, isRef := v.L.(*ast.ColumnNameExpr)
		if !isRef {
			c.AppendErr(NewErrorf(ErrNotSupported,
				"b-op with non column expr on left side(not recommended): %s",
				utils.RestoreNode(v)))
			return "", "", false
		}
		return left.Name.Name.String(), left.Name.Table.String(), true
	case *ast.InsertStmt:
		for i, expr := range v.Lists[0] {
			if expr == n {
				return v.Columns[i].Name.String(), v.Columns[i].Table.String(), true
			}
		}
	case *ast.Assignment:
		return v.Column.Name.String(), v.Column.Table.String(), true
	}

	return "", "", false
}

func (c *ParamExtractVisitor) isInList() bool {
	_, ok := c.FindInCtx((*ast.PatternInExpr)(nil))
	return ok
}

// Enter - Implements Visitor
func (c *ParamExtractVisitor) Enter(n ast.Node) (ast.Node, bool) {
	c.baseVisitor.Enter(n)
	switch v := n.(type) {
	case *driver.ParamMarkerExpr: // interface
		name, table, ok := c.findNameInContext(n)
		if !ok {
			c.AppendErr(NewErrorf(ErrCompilerError, "Failed to infer name of %s",
				utils.RestoreNode(v)))
			return n, true
		}
		isInList := c.isInList()
		c.Params = append(c.Params, GoParam{
			Name:      name,
			TableName: table,
			InPattern: isInList,
			Order:     0, // will be set in the end.
			Marker:    v,
		})
	}
	return n, false
}

// Leave - Implements Visitor
func (c *ParamExtractVisitor) Leave(n ast.Node) (ast.Node, bool) {
	c.baseVisitor.Leave(n)
	if c.IsLeavingRoot() {
		sort.SliceStable(c.Params, func(i, j int) bool {
			return c.Params[i].Marker.Offset < c.Params[j].Marker.Offset
		})
		for i := range c.Params {
			c.Params[i].Order = i
		}
	}
	return n, true
}
