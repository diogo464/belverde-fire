package main

import (
	"os/exec"

	"github.com/pkg/errors"
)

func magickConvert(src string, dst string) error {
	cmd := exec.Command("convert", src, dst)
	err := cmd.Run()
	if err != nil {
		return errors.Wrapf(err, "running convert '%v' '%v'", src, dst)
	}
	return nil
}
