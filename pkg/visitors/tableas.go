package visitors

import (
	"github.com/pingcap/tidb/parser/ast"
)

// TableAsVisitor - after apply, contains a mapping from alias to original table names.
// It will also check table name uniqueness, name shadowing.
type TableAsVisitor struct {
	*baseVisitor
	TableNames map[string]bool
	TableAlias map[string]string
}

// NewTableAsVisitor - @p tableNames defined tables.
func NewTableAsVisitor(tableNames []string) *TableAsVisitor {
	rst := &TableAsVisitor{
		baseVisitor: newBaseVisitor("TableAs"),
		TableNames:  make(map[string]bool),
		TableAlias:  make(map[string]string),
	}
	for _, name := range tableNames {
		rst.TableNames[name] = true
	}
	return rst
}

var _ ast.Visitor = &TableAsVisitor{}

func (c *TableAsVisitor) isAliasUnique(name string) bool {
	_, sameAsTableName := c.TableNames[name]
	_, alreadyUsed := c.TableAlias[name]
	return !sameAsTableName && !alreadyUsed
}

func (c *TableAsVisitor) isTableDefined(name string) bool {
	_, has := c.TableNames[name]
	return has
}

// Enter - Implements Visitor
func (c *TableAsVisitor) Enter(n ast.Node) (ast.Node, bool) {
	if table, ok := n.(*ast.TableSource); ok {
		name, simple := table.Source.(*ast.TableName)
		if !simple {
			// not a simple table, not supported for now
			c.AppendErr(NewError(ErrNotSupported, table.Text()))
			return n, true
		}
		nameStr := name.Name.String()
		if !c.isTableDefined(nameStr) {
			c.AppendErr(NewError(ErrInvalidExpr, "table not defined: "+nameStr))
			return n, true
		}
		aliasStr := table.AsName.String()
		if aliasStr != "" {
			if !c.isAliasUnique(aliasStr) {
				c.AppendErr(NewError(ErrInvalidExpr, "table alias not unique: "+aliasStr))
				return n, true
			}
			c.TableAlias[aliasStr] = nameStr
		}
	}
	return n, false
}

// Leave - Implements Visitor
func (c *TableAsVisitor) Leave(n ast.Node) (ast.Node, bool) {
	return n, true
}
