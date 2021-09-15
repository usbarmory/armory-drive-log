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

package api

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
)

// Checkpoint represents a minimal log checkpoint.
type Checkpoint struct {
	// Origin is the unique identifier for the log issuing this checkpoint.
	Origin string
	// Size is the number of entries in the log at this checkpoint.
	Size uint64
	// Hash is the hash which commits to the contents of the entire log.
	Hash []byte
}

// Unmarshal parses the common formatted checkpoint data and stores the result
// in the Checkpoint.
//
// The supplied data is expected to begin with the following 3 lines of text,
// each followed by a newline:
//  - <Origin string>
//  - <decimal representation of log size>
//  - <base64 representation of root hash>
//
// There must be no extraneous trailing data.
func (c *Checkpoint) Unmarshal(data []byte) error {
	l := bytes.SplitN(data, []byte("\n"), 4)
	if len(l) < 4 {
		return errors.New("invalid checkpoint - too few newlines")
	}
	origin := string(l[0])
	if len(origin) == 0 {
		return fmt.Errorf("invalid checkpoint - empty origin")
	}
	size, err := strconv.ParseUint(string(l[1]), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid checkpoint - size invalid: %w", err)
	}
	h, err := base64.StdEncoding.DecodeString(string(l[2]))
	if err != nil {
		return fmt.Errorf("invalid checkpoint - invalid hash: %w", err)
	}
	if xl := len(l[3]); xl > 0 {
		return fmt.Errorf("invalid checkpoint - %d bytes of unexpected trailing data", xl)
	}
	*c = Checkpoint{
		Origin: origin,
		Size:   size,
		Hash:   h,
	}
	return nil
}
