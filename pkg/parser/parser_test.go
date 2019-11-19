package parser

// import (
// 	"fmt"
// 	"os"
// 	"testing"

// 	"github.com/pingcap/parser/ast"
// 	"github.com/stretchr/testify/suite"

// 	"github.com/stumble/needle/pkg/config"
// )

// type ParserTestSuite struct {
// 	suite.Suite
// 	parser *SQLParser
// }

// func (suite *ParserTestSuite) SetupTest() {
// 	suite.parser = NewSQLParser()
// }

// func (suite *ParserTestSuite) TestParseOneStmtSchema() {
// 	file, err := os.Open("testdata/example.xml")
// 	suite.Require().Nil(err)
// 	defer file.Close()

// 	config, err := config.ParseConfig(file)
// 	suite.Require().Nil(err)

// 	schemaNode, err := suite.parser.ParseOneStmt(config.Schema)
// 	suite.Require().Nil(err)

// 	ddlNode, ok := schemaNode.(ast.DDLNode)
// 	suite.Require().True(ok)

// 	tstmt, ok := ddlNode.(*ast.CreateTableStmt)
// 	suite.Require().True(ok)

// 	suite.False(tstmt.IfNotExists)
// 	// fmt.Println(DeepSprintIR(tstmt))

// 	checked := 0
// 	for _, col := range tstmt.Cols {
// 		// Double name is expected, for .O string
// 		if col.Name.Name.String() == "username" {
// 			for i, v := range col.Options {
// 				if i == 0 {
// 					checked++
// 					suite.Equal(ast.ColumnOptionNotNull, v.Tp)
// 				} else if i == 1 {
// 					checked++
// 					suite.Equal(ast.ColumnOptionPrimaryKey, v.Tp)
// 				}
// 			}
// 		}
// 		if col.Name.Name.String() == "dliveVerified" {
// 			for i, v := range col.Options {
// 				if i == 0 {
// 					checked++
// 					suite.Equal(ast.ColumnOptionNotNull, v.Tp)
// 				} else if i == 1 {
// 					checked++
// 					suite.Equal(ast.ColumnOptionDefaultValue, v.Tp)
// 				}
// 			}
// 		}
// 		if col.Name.Name.String() == "languagePreference" {
// 			suite.Equal(1, len(col.Options))
// 			for i, v := range col.Options {
// 				if i == 0 {
// 					checked++
// 					suite.Equal(ast.ColumnOptionDefaultValue, v.Tp)
// 				}
// 			}
// 		}
// 	}
// 	suite.Equal(5, checked)

// 	checked = 0
// 	for _, c := range tstmt.Constraints {
// 		if c.Name == "users_ibu_1" {
// 			checked++
// 			//XXX(yumin): don't know what does other two Uniq-prefix constraints mean.
// 			suite.Equal(ast.ConstraintUniq, c.Tp)
// 			suite.Equal(1, len(c.Keys))
// 			suite.Equal("email", c.Keys[0].Column.Name.String())
// 		}
// 	}
// 	suite.Equal(1, checked)
// }

// func (suite *ParserTestSuite) TestParseQueries() {
// 	file, err := os.Open("testdata/example.xml")
// 	suite.Require().Nil(err)
// 	defer file.Close()

// 	config, err := config.ParseConfig(file)
// 	suite.Require().Nil(err)
// 	suite.Require().Equal(1, len(config.Stmts.Queries))
// 	// fmt.Printf("%+v", config)
// 	// fmt.Println(config.Stmts.Queries[0].SQL)
// 	stmt, err := suite.parser.ParseOneStmt(config.Stmts.Queries[0].SQL)
// 	suite.Require().Nil(err)

// 	selectStmt, ok := stmt.(*ast.SelectStmt)
// 	suite.Require().True(ok)

// 	fmt.Println(DeepSprintIR(selectStmt))
// }

// func TestLocalTestSuite(t *testing.T) {
// 	suite.Run(t, new(ParserTestSuite))
// }
