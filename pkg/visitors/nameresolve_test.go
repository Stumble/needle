package visitors

import (
	// "fmt"
	"strings"
	"testing"

	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/format"
	"github.com/stretchr/testify/suite"

	"github.com/stumble/needle/pkg/parser"
	"github.com/stumble/needle/pkg/schema"
)

type NameResolveTestSuite struct {
	suite.Suite
}

func (suite *NameResolveTestSuite) createTable(tablesql string) schema.SQLTable {
	p := parser.NewSQLParser()
	tableast, err := p.ParseOneStmt(tablesql)
	suite.Require().Nil(err)
	tableinfo := schema.NewTableInfo(tableast.(*ast.CreateTableStmt), []string{"Address"})
	return tableinfo
}

func (suite *NameResolveTestSuite) TestBasic() {
	p := parser.NewSQLParser()
	sql := `
SELECT o.OrderID, o.EmployeeID, o.OrderDate, c.Region, c.PostalCode, c.Country
FROM Customers c, Orders o
WHERE c.CustomerID = o.CustomerID;
`
	stmt, err := p.ParseOneStmt(sql)
	suite.Require().Nil(err)

	nameResolve := NewNameResolveVisitor(nil)
	stmt.Accept(nameResolve)
	suite.Require().Nil(nameResolve.Errors())
	// TODO(yumin): manually checked, apply test visitor
}

func (suite *NameResolveTestSuite) TestImplicitName() {
	p := parser.NewSQLParser()
	cases := []struct {
		name        string
		sql         string
		db          []string
		expectedErr []error
		expected    string
	}{
		{
			"select",
			`SELECT id, username, ticketNumber, prize, createdAt
		     FROM happyhourwinner
		     WHERE happyHourID = ?;`,
			[]string{`
            CREATE TABLE happyhourwinner (
                id int,
                happyHourID int,
                username varchar(255),
                ticketNumber int,
                prize int,
                createdAt datetime
            );`},
			nil,
			"SELECT `happyhourwinner`.`id`,`happyhourwinner`.`username`,`happyhourwinner`.`ticketNumber`,`happyhourwinner`.`prize`,`happyhourwinner`.`createdAt` FROM `happyhourwinner` WHERE `happyhourwinner`.`happyHourID`=?",
		},
		{
			"update",
			`UPDATE happyhourwinner
		    SET prize = prize + ?
		    WHERE happyHourID = ?;`,
			[]string{`
            CREATE TABLE happyhourwinner (
                id int,
                happyHourID int,
                username varchar(255),
                ticketNumber int,
                prize int,
                createdAt datetime
            );`},
			nil,
			"UPDATE `happyhourwinner` SET `happyhourwinner`.`prize`=`happyhourwinner`.`prize`+? WHERE `happyhourwinner`.`happyHourID`=?",
		},
		{
			"delete",
			`DELETE FROM happyhourwinner
		     WHERE happyHourID = ?;`,
			[]string{`
            CREATE TABLE happyhourwinner (
                id int,
                happyHourID int,
                username varchar(255),
                ticketNumber int,
                prize int,
                createdAt datetime
            );`},
			nil,
			"DELETE FROM `happyhourwinner` WHERE `happyhourwinner`.`happyHourID`=?",
		},
		{
			"insert",
			`INSERT INTO happyhourwinner (happyHourID, happyHourTime, happyHourX)
		     VALUES (?, ?, ?);`,
			[]string{`
            CREATE TABLE happyhourwinner (
                id int,
                happyHourID int,
                username varchar(255),
                ticketNumber int,
                prize int,
                createdAt datetime,
                happyHourX int,
                happyHourTime int
            );`},
			nil,
			"INSERT INTO `happyhourwinner` (`happyhourwinner`.`happyHourID`,`happyhourwinner`.`happyHourTime`,`happyhourwinner`.`happyHourX`) VALUES (?,?,?)",
		},
		{
			"select join",
			`SELECT id, title, prize, user.username, happyhourwinner.username
             FROM user INNER JOIN happyhourwinner ON user.userid = happyhourwinner.id;`,
			[]string{`
            CREATE TABLE happyhourwinner (
                id int,
                happyHourID int,
                username varchar(255),
                ticketNumber int,
                prize int,
                createdAt datetime
            );`,
				`CREATE TABLE user (
                userid int,
                username varchar(333),
                title varchar(333),
                createdAt datetime
            );`,
			},
			nil,
			"SELECT `happyhourwinner`.`id`,`user`.`title`,`happyhourwinner`.`prize`,`user`.`username`,`happyhourwinner`.`username` FROM `user` JOIN `happyhourwinner` ON `user`.`userid`=`happyhourwinner`.`id`",
		},
		{
			"ambiguous names",
			`SELECT id, title, prize, username
             FROM user INNER JOIN happyhourwinner ON user.userid = happyhourwinner.id;`,
			[]string{`
            CREATE TABLE happyhourwinner (
                id int,
                happyHourID int,
                username varchar(255),
                ticketNumber int,
                prize int,
                createdAt datetime
            );`,
				`CREATE TABLE user (
                userid int,
                username varchar(333),
                title varchar(333),
                createdAt datetime
            );`,
			},
			[]error{Error{Type: 1, Detail: "ambiguous expression: username, multiple defs: [user happyhourwinner]"}, Error{Type: 1, Detail: "cannot find the column of (username)"}},
			"",
		},
	}
	for _, t := range cases {
		suite.Run(t.name, func() {
			stmt, err := p.ParseOneStmt(t.sql)
			suite.Require().Nil(err)

			db := make([]schema.SQLTable, 0)
			for _, table := range t.db {
				db = append(db, suite.createTable(table))
			}

			nameResolve := NewNameResolveVisitor(db)
			stmt.Accept(nameResolve)
			suite.Require().Equal(t.expectedErr, nameResolve.Errors())
			if t.expectedErr != nil {
				return
			}

			var sb strings.Builder
			formatCtx := format.NewRestoreCtx(format.DefaultRestoreFlags, &sb)
			_ = stmt.Restore(formatCtx)
			rst := sb.String()
			suite.Equal(t.expected, rst)
		})
	}
}

func TestNameResolveTestSuite(t *testing.T) {
	suite.Run(t, new(NameResolveTestSuite))
}
