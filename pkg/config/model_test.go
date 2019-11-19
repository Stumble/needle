package config

import (
	"fmt"
	// "os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// more test cases.
func TestBasic(t *testing.T) {
	assert := assert.New(t)
	config, err := ParseConfigFromFile("testdata/orders.xml")
	assert.Nil(err)

	fmt.Println(config.Schema)
}
