package parser

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/pingcap/tidb/parser"
	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/mysql"
	_ "github.com/pingcap/tidb/types/parser_driver" // required by TiDB's parser
)

// type StmtNode = ast.StmtNode

// DeepSprintIR return a pretty printed string
func DeepSprintIR(a ast.Node) string {
	return spew.Sdump(a)
}

// SQLParser - parse sql using local parser instance, not goroutine-safe
type SQLParser struct {
	parser *parser.Parser
}

// NewSQLParser - a sql parser
func NewSQLParser() *SQLParser {
	return &SQLParser{
		parser: parser.New(),
	}
}

// ParseOneStmt - parse one statement
// For create table statement, column flags are set.
func (s *SQLParser) ParseOneStmt(stmt string) (ast.StmtNode, error) {
	rst, err := s.parser.ParseOneStmt(stmt, "utf8", "")
	if err != nil {
		return rst, err
	}
	if tb, ok := rst.(*ast.CreateTableStmt); ok {
		setFlags(tb)
	}
	return rst, nil
}

// // Parse - parse statements
// func (s *SQLParser) Parse(stmt string) ([]ast.StmtNode, []error, error) {
// 	return s.parser.Parse(stmt, "utf8", "")
// }

func setFlags(tb *ast.CreateTableStmt) {
	for _, col := range tb.Cols {
		for _, op := range col.Options {
			switch op.Tp {
			case ast.ColumnOptionNotNull:
				col.Tp.Flag |= mysql.NotNullFlag
			case ast.ColumnOptionPrimaryKey:
				col.Tp.Flag |= mysql.PriKeyFlag
			case ast.ColumnOptionAutoIncrement:
				col.Tp.Flag |= mysql.AutoIncrementFlag
			default:
				// TODO(yumin): support all flags
			}
		}
	}
}
