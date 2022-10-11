package config

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type modelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(modelTestSuite))
}

func (suite *modelTestSuite) TestBasic() {
	config, err := ParseConfigFromFile("testdata/orders.xml")
	suite.Require().NoError(err)
	suite.Equal("Orders", config.Schema.Name)
	suite.Equal("Order", config.Schema.MainObj)
}
