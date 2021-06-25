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

	// InclusionProof is the proof of inclusion for FirmwareRelease under NewCheckpoint.
	InclusionProof [][]byte

	// ConsistencyProofs is a map containing consistency proofs from a number of smaller
	// Checkpoints to NewCheckpoint.
	//
	// In the case where the installer is interacting with the device to perform the update,
	// this map can contain a single entry since the installer can interrogate the device
	// to know which Checkpoint is current holds.
	// For non-interactive updates a proof bundle can be provided which contains many/all
	// possible consistency proofs from smaller Checkpoints.
	ConsistencyProofs map[uint64][][]byte
}
