package schema

import (
	// "fmt"
	"testing"

	"github.com/pingcap/tidb/parser/ast"
	"github.com/stretchr/testify/suite"

	"github.com/stumble/needle/pkg/config"
	// "github.com/stumble/needle/pkg/parser"
)

type TableInfoTestSuite struct {
	suite.Suite
}

func (suite *TableInfoTestSuite) TestBasic() {
	config, err := config.ParseConfigFromFile("testdata/schema1.xml")
	suite.Require().Nil(err)

	schemaNode, err := config.Schema.SQL.Parse()
	suite.Require().Nil(err)
	tstmt, ok := schemaNode.(*ast.CreateTableStmt)
	suite.Require().True(ok)

	tableInfo := NewTableInfo(tstmt, []string{})
	suite.Equal("users", tableInfo.Name())

	results := []struct {
		u  string
		nn bool
		t  GoType
	}{
		{"uname", true, GoType{GoTypeString, true}},
		{"dname", true, GoType{GoTypeString, true}},
		{"changed", true, GoType{GoTypeTime, true}},
		{"verified", true, GoType{GoTypeBool, true}},
		{"languagePreference", false, GoType{GoTypeString, false}},
		{"avatar", false, GoType{GoTypeString, false}},
		{"about", false, GoType{GoTypeString, false}},
		{"email", false, GoType{GoTypeString, false}},
		{"followerCount", false, GoType{GoTypeInt, false}},
		{"createdAt", true, GoType{GoTypeTime, true}},
		{"watching", true, GoType{GoTypeInt, true}},
	}

	for i, v := range tableInfo.Columns() {
		suite.Equal(results[i].u, v.Name())
		suite.Equal(results[i].nn, v.NotNull())
		suite.Equal(results[i].t, v.GoType())
	}
}

func TestTableInfoTestSuite(t *testing.T) {
	suite.Run(t, new(TableInfoTestSuite))
}
