package passes

import (
	"github.com/stumble/needle/pkg/driver"
)

// Pass - is a series of actions(visitors) applied on AST.
// Visitor Errors should be handled inside Pass. Caller of Run only
// need to check error == nil, then panic if false.
type Pass interface {
	Run(repo *driver.Repo) error
}
