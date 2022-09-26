package visitors

import (
	// "fmt"
	"testing"

	// "github.com/pingcap/tidb/parser/ast"
	"github.com/stretchr/testify/suite"

	"github.com/stumble/needle/pkg/config"
	"github.com/stumble/needle/pkg/driver"
	// "github.com/stumble/needle/pkg/parser"
)

// To test type inference,
type TypeInferenceTestSuite struct {
	suite.Suite
}

func (suite *TypeInferenceTestSuite) TestBasic() {
	customers, err := config.ParseConfigFromFile("testdata/orders.xml")
	suite.Require().Nil(err)

	repo, err := driver.NewRepoFromConfig(customers)
	suite.Require().Nil(err)

	for _, v := range repo.Queries {
		ti := NewTypeInferenceVisitor(repo.Tables, make(map[string]string))
		v.Node.Accept(ti)
		suite.Require().Nil(ti.Errors())
	}
}

func TestTypeInferenceTestSuite(t *testing.T) {
	suite.Run(t, new(TypeInferenceTestSuite))
}
