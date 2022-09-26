package visitors

import (
	"testing"

	"github.com/pingcap/tidb/parser/ast"
	"github.com/stretchr/testify/suite"

	"github.com/stumble/needle/pkg/parser"
	"github.com/stumble/needle/pkg/schema"
	"github.com/stumble/needle/pkg/utils"
)

type StarElimTestSuite struct {
	suite.Suite
}

func (suite *StarElimTestSuite) TestStarElimBasic() {
	p := parser.NewSQLParser()
	tablesql := `
        CREATE TABLE Persons (
            PersonID int,
            LastName varchar(255),
            FirstName varchar(255),
            Address varchar(255),
            City varchar(255)
        );`
	tableast, err := p.ParseOneStmt(tablesql)
	suite.Require().Nil(err)

	tableinfo := schema.NewTableInfo(tableast.(*ast.CreateTableStmt), []string{"Address"})
	suite.Require().Nil(tableinfo.Valid())

	sql := `SELECT * FROM Persons`
	stmt, err := p.ParseOneStmt(sql)
	suite.Require().Nil(err)

	// apply table as for input
	starElim := NewStarElimVisitor(tableinfo)
	stmt.Accept(starElim)
	suite.Require().Nil(starElim.Errors())

	suite.Equal("SELECT PersonID,LastName,FirstName,City FROM Persons",
		utils.RestoreNode(stmt))
}

func TestStarElimTestSuite(t *testing.T) {
	suite.Run(t, new(StarElimTestSuite))
}
