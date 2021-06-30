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

// verify_release is a tool to verify a release manifest.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
	"golang.org/x/mod/sumdb/note"
)

var (
	publicKeyFile = flag.String("public_key", "", "Path to file containing the public key used to sign the manifest")
	manifest      = flag.String("manifest", "", "Path to the signed manifest")
)

func main() {
	flag.Parse()
	if err := validateFlags(); err != nil {
		glog.Exitf("Invalid flag(s):\n%s", err)
	}

	msg, err := os.ReadFile(*manifest)
	if err != nil {
		glog.Exitf("failed to read manifest file: %v", err)
	}

	glog.Info("Verifying signature...")
	body, err := verify(msg)
	if err != nil {
		glog.Exitf("Failed to verify signature: %v", err)
	}

	fmt.Println(string(body))
}

// verify verifies the passed Go sumdb's note
func verify(msg []byte) ([]byte, error) {
	k, err := os.ReadFile(*publicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %v", err)
	}

	verifier, err := note.NewVerifier(string(k))
	if err != nil {
		return nil, fmt.Errorf("failed to initialise key: %v", err)
	}
	verifiers := note.VerifierList(verifier)

	n, err := note.Open(msg, verifiers)
	if err != nil {
		return nil, fmt.Errorf("failed to verify manifest: %v", err)
	}

	return []byte(n.Text), nil
}

func validateFlags() error {
	errs := make([]string, 0)
	checkEmpty := func(n, s string) {
		if s == "" {
			errs = append(errs, fmt.Sprintf("--%s can't be empty", n))
		}
	}
	checkEmpty("public_key", *publicKeyFile)
	checkEmpty("manifest", *manifest)

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}
