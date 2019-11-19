package testsuite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type metaTestSuite struct {
	*MysqlTestSuite
}

func NewMetaTestSuite() *metaTestSuite {
	return &metaTestSuite{
		MysqlTestSuite: NewMysqlTestSuite("metaTestDB", []string{
			`CREATE TABLE IF NOT EXISTS docs (
             id int(6) unsigned NOT NULL,
             rev int(3) unsigned NOT NULL,
             content varchar(200) NOT NULL,
             PRIMARY KEY (id,rev)
             ) DEFAULT CHARSET=utf8;`,
		}),
	}
}

func TestMetaTestSuite(t *testing.T) {
	suite.Run(t, NewMetaTestSuite())
}

func (suite *metaTestSuite) SetupTest() {
	suite.MysqlTestSuite.SetupTest()
}

func (suite *metaTestSuite) TestInsertQuery() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exec := suite.Manager.GetDBExecuter()
	rst, err := exec.Exec(ctx, "INSERT INTO `docs` (`id`, `rev`, `content`) VALUES (?,?,?)", 33, 44, "hahaha")
	suite.Nil(err)
	n, err := rst.RowsAffected()
	suite.Nil(err)
	suite.Equal(int64(1), n)

	exec = suite.MysqlTestSuite.Manager.GetDBExecuter()
	rows, err := exec.Query(ctx, "SELECT `content` FROM `docs` WHERE `id` = ?", 33)
	suite.Nil(err)

	content := ""
	suite.True(rows.Next())
	err = rows.Scan(&content)
	suite.Nil(err)
	suite.Equal("hahaha", content)
}
