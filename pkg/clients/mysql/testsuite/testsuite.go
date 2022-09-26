package testsuite

import (
	"context"
	"fmt"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/stumble/needle/pkg/clients/mysql"
)

type MysqlTestSuite struct {
	suite.Suite
	Testdb  string
	Tables  []string
	Config  *mysql.Config
	Manager mysql.Manager
}

// NewMysqlTestSuite @p db is the name of test db and tables are table creation
// SQL statements. DB will be created, so does tables, on SetupTest.
// If you pass different @p db for suites in different packages, you can test them in parallel.
func NewMysqlTestSuite(db string, tables []string) *MysqlTestSuite {
	config := mysql.ConfigFromEnv()
	config.DBName = db
	return NewMysqlTestSuiteWithConfig(config, db, tables)
}

func NewMysqlTestSuiteWithConfig(config *mysql.Config, db string, tables []string) *MysqlTestSuite {
	return &MysqlTestSuite{
		Testdb: db,
		Tables: tables,
		Config: config,
	}
}

func (suite *MysqlTestSuite) SetupTest() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// create DB
	conn, err := mysql.RawMysqlConn(suite.Config)
	suite.Require().Nil(err)
	defer conn.Close()
	_, err = conn.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s;", suite.Testdb))
	if err != nil {
		panic(err)
	}
	_, err = conn.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s;", suite.Testdb))
	if err != nil {
		panic(err)
	}

	// create manager
	manager, err := mysql.NewMysqlManager(suite.Config)
	if err != nil {
		panic(err)
	}
	suite.Manager = manager

	// create tables
	for _, v := range suite.Tables {
		exec := suite.Manager.GetDBExecuter()
		_, err := exec.Exec(ctx, v)
		if err != nil {
			panic(err)
		}
	}
}
