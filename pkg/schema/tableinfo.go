package schema

import (
	"errors"

	"github.com/pingcap/tidb/parser/ast"

	"github.com/stumble/needle/pkg/utils"
)

// TableInfo - a table.
type TableInfo struct {
	stmt         *ast.CreateTableStmt
	hiddenFields []string
}

// NewTableInfo -
func NewTableInfo(s *ast.CreateTableStmt, hiddenFields []string) SQLTable {
	// XXX(yumin): enforce if not exists in table create seems to be a good idea,
	// as mysql workbench does this when export db schema.
	s.IfNotExists = true
	return &TableInfo{
		stmt:         s,
		hiddenFields: hiddenFields,
	}
}

// Valid return nil if valid
func (t *TableInfo) Valid() error {
	colNames := make(map[string]bool)
	for _, col := range t.stmt.Cols {
		_, has := colNames[col.Name.Name.String()]
		if has {
			return errors.New("duplicated column name: " + col.Name.Name.String())
		}
		colNames[col.Name.Name.String()] = true
	}

	for _, hf := range t.hiddenFields {
		_, has := colNames[hf]
		if !has {
			return errors.New("hidden fields not defined in table: " + hf)
		}
	}
	return nil
}

// Name - implements SQLTable
func (t *TableInfo) Name() string {
	return t.stmt.Table.Name.String()
}

// SQL - implements SQLTable
func (t *TableInfo) SQL() string {
	return utils.RestoreNode(t.stmt)
}

// // Node - implements SQLTable
// func (t *TableInfo) Node() *ast.CreateTableStmt {
// 	return t.stmt
// }

func makeColumns(s *ast.CreateTableStmt) (rst []SQLColumn) {
	for _, col := range s.Cols {
		rst = append(rst, NewColumnInfo(col))
	}
	return
}

// Columns - implements SQLTable
func (t *TableInfo) Columns() []SQLColumn {
	return makeColumns(t.stmt)
}

// StarColumns - implements SQLTable
func (t *TableInfo) StarColumns() (rst []SQLColumn) {
	hfNames := make(map[string]bool)
	for _, v := range t.hiddenFields {
		hfNames[v] = true
	}
	for _, col := range t.stmt.Cols {
		_, hidden := hfNames[col.Name.Name.String()]
		if !hidden {
			rst = append(rst, NewColumnInfo(col))
		}
	}
	return
}

// Indexes - implements SQLTable
// Should include constraints and keys with pk option.
func (t *TableInfo) Indexes() (rst []SQLIndex) {
	for _, c := range t.stmt.Constraints {
		rst = append(rst, NewIndexInfo(c))
	}
	return
}
