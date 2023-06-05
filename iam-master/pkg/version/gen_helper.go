// Copyright 2019 The OpenPitrix Authors. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

// +build ignore

package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"
)

func main() {
	version, gitSha1Version := getAppVersion()
	buildTime := time.Now().Format("2006-01-02 15:04:05")

	data := make_update_version_go_file(version, gitSha1Version, buildTime)

	err := ioutil.WriteFile("./z_update_version.go", []byte(data), 0666)
	if err != nil {
		fmt.Printf("ioutil.WriteFile: err = %+v", err)
	}

	fmt.Printf("%s (%s)\n", version, gitSha1Version)
	fmt.Println(buildTime)
}

func make_update_version_go_file(version, gitSha1Version, buildTime string) string {
	return fmt.Sprintf(`
// Copyright 2019 The OpenPitrix Authors. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

// Auto generated by 'go run gen_helper.go', DO NOT EDIT.

package version

func init() {
	ShortVersion = "%s"
	GitSha1Version = "%s"
	BuildDate = "%s"
}
`,
		version,
		gitSha1Version,
		buildTime,
	)
}

func getAppVersion() (version, gitSha1Version string) {
	// VERSION=`git describe --tags --always --dirty="-dev"`
	versionData, err := exec.Command(
		`git`, `describe`, `--tags`, `--always`, `--dirty=-dev`,
	).CombinedOutput()
	if err != nil {
		fmt.Printf("%+v", err)
	}

	// GIT_SHA1=`git show --quiet --pretty=format:%H`
	gitSha1VersionData, err := exec.Command(
		`git`, `show`, `--quiet`, `--pretty=format:%H`,
	).CombinedOutput()
	if err != nil {
		fmt.Printf("%+v", err)
	}

	version = strings.TrimSpace(string(versionData))
	gitSha1Version = strings.TrimSpace(string(gitSha1VersionData))
	return
}
