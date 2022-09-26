package visitors

import (
	"testing"

	// "github.com/pingcap/tidb/parser/ast"
	"github.com/stretchr/testify/suite"

	"github.com/stumble/needle/pkg/parser"
)

type tableAsTestSuite struct {
	suite.Suite
}

func (suite *tableAsTestSuite) TestBasic() {
	parser := parser.NewSQLParser()
	sql := "SELECT email as e FROM users as uuuu WHERE `username`=? AND `totalRank` > 10;"
	stmt, err := parser.ParseOneStmt(sql)
	suite.Require().Nil(err)

	visitor := NewTableAsVisitor([]string{"users"})
	stmt.Accept(visitor)
	suite.Require().Nil(visitor.Errors())
	suite.Equal(1, len(visitor.TableNames))
	suite.Equal(true, visitor.TableNames["users"])
	suite.Equal(1, len(visitor.TableAlias))
	suite.Equal("users", visitor.TableAlias["uuuu"])
}

func (suite *tableAsTestSuite) TestImplicitAsPlusJoin() {
	p := parser.NewSQLParser()
	sql := `
SELECT o.OrderID, o.EmployeeID, o.OrderDate, c.Region, c.PostalCode, c.Country
FROM Customers c, Orders o
WHERE c.CustomerID = o.CustomerID;
`
	stmt, err := p.ParseOneStmt(sql)
	suite.Require().Nil(err)

	visitor := NewTableAsVisitor([]string{"Customers", "Orders"})
	stmt.Accept(visitor)
	suite.Require().Nil(visitor.Errors())
	suite.Equal(2, len(visitor.TableNames))
	suite.Equal(true, visitor.TableNames["Customers"])
	suite.Equal(true, visitor.TableNames["Orders"])

	suite.Equal(2, len(visitor.TableAlias))
	suite.Equal("Customers", visitor.TableAlias["c"])
	suite.Equal("Orders", visitor.TableAlias["o"])
}

func (suite *tableAsTestSuite) TestJoin() {
	p := parser.NewSQLParser()
	sql := `
	SELECT p.permlink, u.username, l.time
	FROM livestreams l
	INNER JOIN posts p ON l.permlink = p.permlink
	INNER JOIN users u ON l.username = u.username
	WHERE l.username = ?
`

	stmt, err := p.ParseOneStmt(sql)
	suite.Require().Nil(err)

	visitor := NewTableAsVisitor([]string{"users", "posts", "livestreams"})
	stmt.Accept(visitor)
	suite.Require().Nil(visitor.Errors())

	suite.Equal(3, len(visitor.TableAlias))
	suite.Equal("users", visitor.TableAlias["u"])
	suite.Equal("livestreams", visitor.TableAlias["l"])
	suite.Equal("posts", visitor.TableAlias["p"])
}

func (suite *tableAsTestSuite) TestTableUndefined() {
	p := parser.NewSQLParser()
	sql := `
	SELECT p.permlink, u.username, l.time
	FROM livestreams l
	INNER JOIN posts p ON l.permlink = p.permlink
	INNER JOIN users u ON l.username = u.username
	WHERE l.username = ?
`

	stmt, err := p.ParseOneStmt(sql)
	suite.Require().Nil(err)

	visitor := NewTableAsVisitor([]string{})
	stmt.Accept(visitor)
	suite.NotNil(visitor.Errors())
}

func TestTableAsTestSuite(t *testing.T) {
	suite.Run(t, new(tableAsTestSuite))
}
