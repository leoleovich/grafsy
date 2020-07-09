// +build !noacl

package grafsy

import (
	"github.com/naegelejd/go-acl"
	"github.com/pkg/errors"
)

func setACL(metricDir string) error {
	ac, err := acl.Parse("user::rw group::rw mask::r other::r")
	if err != nil {
		return errors.New("Unable to parse acl: " + err.Error())
	}
	err = ac.SetFileDefault(metricDir)
	if err != nil {
		return errors.New("Unable to set acl: " + err.Error())
	}
	return nil
}
