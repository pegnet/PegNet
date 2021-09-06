// Copyright (c) of parts are held by the various contributors (see the CLA)
// Licensed under the MIT License. See LICENSE file in the project root for full
package common

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zpatrick/go-config"
)

var PointMultiple float64 = 100000000

type NetworkType string

// LoadConfigNetwork handles the different casings of `MainNet`.
//	So: `mainnet`, `Mainnet`, and other stuff is all valid
func LoadConfigNetwork(c *config.Config) (string, error) {
	network, err := c.String(ConfigPegnetNetwork)
	if err != nil {
		return "", err
	}
	return GetNetwork(network)
}

func LoadConfigStakerNetwork(c *config.Config) (string, error) {
	network, err := c.String(ConfigPegnetStakeNetwork)
	if err != nil {
		return "", err
	}
	return GetNetwork(network)
}

func LoadDelegatorsSignatures(c *config.Config, delegatee string) []byte {
	delegateeAddresses, err := c.String("DelegateStaker.DelagateeAddress")
	if err != nil {
		return nil
	}
	delegatorList, err := c.String("DelegateStaker.DelegatorList")
	if err != nil {
		return nil
	}

	delegateeAddrs := strings.Split(delegateeAddresses, ",")
	delegatorsPaths := strings.Split(delegatorList, ",")

	var dPath = ""
	for i := 0; i < len(delegateeAddrs); i++ {
		if delegateeAddrs[i] == delegatee {
			dPath = delegatorsPaths[i]
			break
		}
	}

	// Read signature data from file
	path := os.ExpandEnv(dPath)
	f, err := os.Open(path)
	if err != nil {
		return nil
	}

	defer f.Close()
	scanner := bufio.NewScanner(f)

	var delegatorsSigResult []byte
	for scanner.Scan() {
		sigData := scanner.Text()
		sigDataStr := strings.Split(sigData, " ")
		var byteArray []byte
		for i := 0; i < len(sigDataStr); i++ {
			i, _ := strconv.Atoi(sigDataStr[i])
			byteArray = append(byteArray, byte(i))
		}
		fmt.Println(byteArray)
		delegatorsSigResult = append(delegatorsSigResult[:], byteArray[:]...)
	}

	if err := scanner.Err(); err != nil {
		return nil
	}

	return delegatorsSigResult
}

func GetNetwork(network string) (string, error) {
	switch strings.ToLower(network) {
	case strings.ToLower(MainNetwork):
		return MainNetwork, nil
	case strings.ToLower(TestNetwork), strings.ToLower("TestNet"):
		return TestNetwork, nil
	case strings.ToLower(UnitTestNetwork), strings.ToLower("UnitTest"):
		return UnitTestNetwork, nil
	default:
		return "", fmt.Errorf("'%s' is not a valid network", network)
	}
}

const (
	ZeroHash = "0000000000000000000000000000000000000000000000000000000000000000"
)

const (
	MainNetwork     = "MainNet"
	TestNetwork     = "TestNet-pM7"
	UnitTestNetwork = "unit-test" // Only used for unit tests

	MainNetworkRCD = MainNetwork + "RCD"
	TestNetworkRCD = TestNetwork + "RCD"
)

const (
	TransactionChainTag = "Transactions"
	MinerChainTag       = "Miners"
	OPRChainTag         = "OraclePriceRecords"
	SPRChainTag         = "StakingPriceRecords"
)

var (
	// Pegnet Burn Addresses
	BurnAddresses = map[string]string{
		MainNetwork:    "EC2BURNFCT2PEGNETooo1oooo1oooo1oooo1oooo1oooo19wthin",
		TestNetwork:    "EC2BURNFCT2TESTxoooo1oooo1oooo1oooo1oooo1oooo1EoyM6d",
		MainNetworkRCD: "37399721298d77984585040ea61055377039a4c3f3e2cd48c46ff643d50fd64f",
		TestNetworkRCD: "37399721298d8b92934b4f767a56be38ad8a30cf0b7ed9d9fd2eb0919905c4af",
	}
)

func PegnetBurnAddress(network string) string {
	return BurnAddresses[network]
}

var (
	fcPubPrefix = []byte{0x5f, 0xb1}
	fcSecPrefix = []byte{0x64, 0x78}
	ecPubPrefix = []byte{0x59, 0x2a}
	ecSecPrefix = []byte{0x5d, 0xb6}
)

// FormatDiff returns a human readable string in scientific notation
func FormatDiff(diff uint64, precision uint) string {
	format := "%." + fmt.Sprint(precision) + "e"
	return fmt.Sprintf(format, float64(diff))
}

// FormatGrade returns a human readable string in scientific notation
func FormatGrade(grade float64, precision uint) string {
	format := "%." + fmt.Sprint(precision) + "e"
	return fmt.Sprintf(format, grade)
}
