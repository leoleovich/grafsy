//go:build !noacl

package grafsy

import (
	"fmt"

	"github.com/naegelejd/go-acl"
)

func setACL(metricDir string) error {
	ac, err := acl.Parse("user::rw group::rw mask::r other::r")
	if err != nil {
		return fmt.Errorf("unable to parse acl: %w", err)
	}
	err = ac.SetFileDefault(metricDir)
	if err != nil {
		return fmt.Errorf("unable to set acl: %w", err)
	}
	return nil
}
