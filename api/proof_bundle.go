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

// ProofBundle is written to the armory at update time so that the running
// firmware can convince itself of the discoverability of the update before
// installing it.
type ProofBundle struct {
	// NewCheckpoint is the signed checkpoint from the log covering the updated release.
	//
	// This is stored in the format specified at https://github.com/google/trillian-examples/tree/master/formats/log
	NewCheckpoint []byte

	// FirmwareRelease is the signed FirmwareRelease statement corresponding to the update.
	//
	// This is stored as a sumbdb signed note containing the JSON representation of the
	// FirmwareRelease struct.
	FirmwareRelease []byte

	// LeafHashes contains all leaf hashes committed to by NewCheckpoint.
	//
	// This is to allow users who don't/cannot use a tool to install the firmware to verify
	// consistency with any possible Checkpoint they may have on their device currently.
	LeafHashes [][]byte
}
