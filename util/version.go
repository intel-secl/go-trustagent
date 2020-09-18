/*
 * Copyright (C) 2020 Intel Corporation
 * SPDX-License-Identifier: BSD-3-Clause
 */
package util

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	TIME_DEFAULT = "1970-01-01T00:00:00-07:00"
)

var Version = "0.0.0"
var GitHash = "fffffff"
var BuildDate = TIME_DEFAULT
var Branch = ""

type VersionInfo struct {
	Major         int       `json:"major"`
	Minor         int       `json:"minor"`
	Patch         int       `json:"patch"`
	Commit        string    `json:"commit"`
	Built         time.Time `json:"built"`
	VersionString string    `json:"version_string"`
}

var versionInfo *VersionInfo

func parseMajorVersion() (int, error) {
	log.Trace("util/version:GetMajorVersion() Entering")
	defer log.Trace("util/version:GetMajorVersion() Leaving")

	endIdx := strings.Index(Version, ".")
	if endIdx <= 0 {
		return 0, errors.Errorf("util/version:GetMajorVersion() Could not parse version string %s", Version)
	}

	major, err := strconv.Atoi(strings.Replace(Version[0:endIdx], "v", "", -1))
	if err != nil {
		return 0, err
	}

	return major, nil
}

func parseMinorVersion() (int, error) {
	log.Trace("util/version:GetMinorVersion() Entering")
	defer log.Trace("util/version:GetMinorVersion() Leaving")

	startIdx := strings.Index(Version, ".")
	if startIdx <= 0 {
		return 0, errors.Errorf("util/version:GetMinorVersion() Could not parse version string %s", Version)
	}

	endIdx := strings.Index(Version[startIdx+1:], ".")
	if endIdx <= 0 {
		return 0, errors.Errorf("util/version:GetMinorVersion() Could not parse version string %s", Version)
	}

	endIdx += startIdx + 1

	minor, err := strconv.Atoi(Version[startIdx+1 : endIdx])
	if err != nil {
		return 0, err
	}

	return minor, nil
}

func parsePatchVersion() (int, error) {
	log.Trace("util/version:GetPatchVersion() Entering")
	defer log.Trace("util/version:GetPatchVersion() Leaving")

	startIdx := strings.LastIndex(Version, ".")
	if startIdx <= 0 {
		return 0, errors.Errorf("util/version:GetPatchVersion() Could not parse version string %s", Version)
	}

	patch, err := strconv.Atoi(Version[startIdx+1:])
	if err != nil {
		return 0, err
	}

	return patch, nil
}

func GetVersionInfo() (*VersionInfo, error) {
	var err error

	if versionInfo == nil {
		vi := VersionInfo{}

		vi.Major, _ = parseMajorVersion()
		vi.Minor, _ = parseMinorVersion()
		vi.Patch, _ = parsePatchVersion()

		vi.Commit = GitHash

		vi.Built, err = time.Parse(time.RFC3339, BuildDate)
		if err != nil {
			vi.Built, _ = time.Parse(time.RFC3339, TIME_DEFAULT)
		}

		vi.VersionString = fmt.Sprintf("Trust Agent %s-%s\nBuilt %s\n", Version, GitHash, BuildDate)
		if Branch != "" {
			vi.VersionString += fmt.Sprintf("Branch '%s'\n", Branch)
		}

		versionInfo = &vi
	}

	return versionInfo, nil
}
