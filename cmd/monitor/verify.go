// Copyright 2021 The Project Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/f-secure-foundry/armory-drive-log/api"
	"github.com/f-secure-foundry/armory-drive-log/keys"
	"github.com/golang/glog"
)

const (
	gitOwner = "f-secure-foundry"
	gitRepo  = "armory-drive"
)

func NewReproducibleBuildVerifier(cleanup bool) (*ReproducibleBuildVerifier, error) {
	return &ReproducibleBuildVerifier{
		cleanup: cleanup,
	}, nil
}

type ReproducibleBuildVerifier struct {
	cleanup bool
}

func (v *ReproducibleBuildVerifier) VerifyManifest(ctx context.Context, i uint64, r api.FirmwareRelease) error {
	glog.V(1).Infof("VerifyManifest %d: %q", i, r.Revision)
	// Create temporary directory that will be cleaned up after this method returns
	dir, err := os.MkdirTemp("", "armory-verify")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	if v.cleanup {
		defer os.RemoveAll(dir)
	} else {
		glog.Infof("Cleanup disabled: %q will not be deleted after use", dir)
	}

	glog.V(1).Infof("Cloning repo into %q", dir)
	// Clone the repository at the release tag
	cmd := exec.Command("/usr/bin/git", "clone", fmt.Sprintf("https://github.com/%s/%s", gitOwner, gitRepo), "-b", r.Revision)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone: %v (%s)", err, out)
	}

	repoRoot := filepath.Join(dir, gitRepo)
	// Confirm that the git revision matches the manifest
	cmd = exec.Command("/usr/bin/git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get HEAD revision: %v (%s)", err, out)
	}
	if got, want := strings.TrimSpace(string(out)), r.BuildArgs["REV"]; got != want {
		return fmt.Errorf("expected revision %q but got %q for tag %q", want, got, r.Revision)
	}

	// TODO: support downloading other TAMAGO compiler builds.
	// For now, this just uses the one version pointed to by the process env.
	tamagoBin := ""
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "TAMAGO=") {
			tamagoBin = e[len("TAMAGO="):]
		}
	}
	if len(tamagoBin) == 0 {
		return fmt.Errorf("failed to find TAMAGO in env")
	}
	out, err = exec.Command(tamagoBin, "version").Output()
	if err != nil {
		return fmt.Errorf("failed to get tamago version: %v (%s)", err, out)
	}
	if got, want := fmt.Sprintf("tama%s", strings.TrimSpace(string(out))), r.ToolChain; got != want {
		return fmt.Errorf("expected toolchain %q but got %q for tag %q", want, got, r.Revision)
	}

	// Copy the public keys into place
	otaDir := filepath.Join(repoRoot, "internal", "ota")
	if err := os.WriteFile(filepath.Join(otaDir, "armory-drive-log.pub"), []byte(keys.ArmoryDriveLogPub), 0666); err != nil {
		return fmt.Errorf("failed to write key: %v", err)
	}
	if err := os.WriteFile(filepath.Join(otaDir, "armory-drive.pub"), []byte(keys.ArmoryDrivePub), 0666); err != nil {
		return fmt.Errorf("failed to write key: %v", err)
	}

	// Make the imx file
	glog.V(1).Infof("Running make in %s", repoRoot)
	cmd = exec.Command("/usr/bin/make", "CROSS_COMPILE=arm-none-eabi-", "imx")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to make: %v (%s)", err, out)
	}

	// Hash the firmware artifact.
	data, err := ioutil.ReadFile(filepath.Join(repoRoot, api.FirmwareArtifactName))
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", api.FirmwareArtifactName, err)
	}
	if got, want := sha256.Sum256(data), r.ArtifactSHA256[api.FirmwareArtifactName]; !bytes.Equal(got[:], want) {
		// TODO: report this in a more visible way than an error in the log.
		glog.Errorf("Failed to verify %s build (got %x, wanted %x)", api.FirmwareArtifactName, got, want)
	} else {
		glog.Infof("Leaf %d for revision %q verified at git tag %q", i, r.Revision, r.BuildArgs["REV"])
	}

	return nil
}
