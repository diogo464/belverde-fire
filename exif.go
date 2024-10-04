package main

import (
	"encoding/json"
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
)

type ExifData struct {
	Latitude  float64
	Longitude float64
}

func exiftool(path string) (*ExifData, error) {
	cmd := exec.Command("exiftool", "-c", "%+.24f", "-j", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "running exiftool")
	}

	type Output struct {
		GPSLatitude  string
		GPSLongitude string
	}

	os := []Output{}
	if err := json.Unmarshal(output, &os); err != nil || len(os) != 1 {
		return nil, errors.Wrapf(err, "parsing exiftool output")
	}
	o := os[0]

	latitude, err := strconv.ParseFloat(o.GPSLatitude, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing latitude '%v'", o.GPSLatitude)
	}

	longitude, err := strconv.ParseFloat(o.GPSLongitude, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing longitude '%v'", o.GPSLongitude)
	}

	return &ExifData{Latitude: latitude, Longitude: longitude}, nil
}
