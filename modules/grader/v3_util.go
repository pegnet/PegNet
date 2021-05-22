package grader

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"regexp"

	"github.com/pegnet/pegnet/modules/factoidaddress"
	"github.com/pegnet/pegnet/modules/opr"
)

// ValidateV2 validates the provided data using the specified parameters
func ValidateV3(entryhash []byte, extids [][]byte, height int32, winners []string, content []byte) (*GradingOPR, error) {
	if len(entryhash) != 32 {
		return nil, NewValidateError("invalid entry hash length")
	}

	if len(extids) != 3 {
		return nil, NewValidateError("invalid extid count")
	}

	if len(extids[1]) != 8 {
		return nil, NewValidateError("self reported difficulty must be 8 bytes")
	}

	if len(extids[2]) != 1 || extids[2][0] != 3 {
		return nil, NewValidateError("invalid version")
	}

	o, err := opr.ParseV2Content(content)
	if err != nil {
		return nil, NewDecodeError(err.Error())
	}

	if o.Height != height {
		return nil, NewValidateError("invalid height")
	}

	// verify assets
	if len(o.Assets) != len(opr.V2Assets) {
		return nil, NewValidateError("invalid assets")
	}
	for _, val := range o.Assets {
		if val == 0 {
			return nil, NewValidateError("assets must be greater than 0")
		}
	}

	if len(o.Winners) != 25 {
		return nil, NewValidateError("must have exactly 25 previous winning shorthashes")
	}

	if err := factoidaddress.Valid(o.Address); err != nil {
		return nil, NewValidateError(fmt.Sprintf("factoidaddress is invalid : %s", err.Error()))
	}

	if valid, _ := regexp.MatchString("^[a-zA-Z0-9,]+$", o.ID); !valid {
		return nil, NewValidateError("only alphanumeric characters and commas are allowed in the identity")
	}

	if !verifyWinnerFormat(o.GetPreviousWinners(), 25) {
		return nil, NewValidateError("incorrect amount of previous winners")
	}

	if !verifyWinners(o.GetPreviousWinners(), winners) {
		return nil, NewValidateError("incorrect set of previous winners")
	}

	gopr := new(GradingOPR)
	gopr.EntryHash = entryhash
	gopr.Nonce = extids[0]
	gopr.SelfReportedDifficulty = binary.BigEndian.Uint64(extids[1])

	sha := sha256.Sum256(content)
	gopr.OPRHash = sha[:]

	gopr.OPR = o

	return gopr, nil
}
