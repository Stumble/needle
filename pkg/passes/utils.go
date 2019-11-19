package passes

import (
	"fmt"
)

func mergeErrors(errs []error) error {
	errStr := ""
	for _, e := range errs {
		errStr += e.Error() + "\n"
	}
	return fmt.Errorf("%s", errStr)
}
