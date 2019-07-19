// Copyright (c) of parts are held by the various contributors (see the CLA)
// Licensed under the MIT License. See LICENSE file in the project root for full license information.

package common

import (
	"crypto/sha256"
)

//ComputeChainIDFromFields
// Takes the binary fields that define a chainID and hashes
// them together to create the ChainID expected by the APIs.
// These fields are treated as binary, but could be simple text, like:
// "Bob's" "Favorite" "Chain"
func ComputeChainIDFromFields(fields [][]byte) []byte {
	hs := sha256.New()
	for _, id := range fields {
		h := sha256.Sum256(id)
		hs.Write(h[:])
	}
	chainID := hs.Sum(nil)
	return chainID
}

// ComputeChainIDFromStrings
// Take a set of strings, and compute the chainID.  If you have binary fields, you
// can call factom.ComputeChainIDFromFields directly.
func ComputeChainIDFromStrings(fields []string) []byte {
	var binary [][]byte
	for _, str := range fields {
		binary = append(binary, []byte(str))
	}
	return ComputeChainIDFromFields(binary)
}
