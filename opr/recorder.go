package opr

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/pegnet/pegnet/balances"

	"github.com/pegnet/pegnet/database"

	"github.com/pegnet/pegnet/common"

	log "github.com/sirupsen/logrus"
	"github.com/zpatrick/go-config"
)

var recLog = log.WithFields(log.Fields{"id": "recorder"})

// ChainRecorder is a tool to create csvs to look at things on chain
type ChainRecorder struct {
	config   *config.Config
	filepath string
}

func NewChainRecorder(con *config.Config, filpath string) (*ChainRecorder, error) {
	c := new(ChainRecorder)
	c.config = con
	c.filepath = filpath

	return c, nil
}

func (c *ChainRecorder) WritePriceCSV(db database.IDatabase, height int64) error {
	InitLX() // We intend to use the LX hash

	f, err := os.OpenFile(c.filepath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	file := f
	writer := csv.NewWriter(file)

	b := balances.NewBalanceTracker()
	g := NewQuickGrader(c.config, db, b)
	var _ = g.Sync()

	block := g.OprBlockByHeight(height)
	if block == nil {
		return fmt.Errorf("%d block is nil", height)
	}
	recLog.WithField("height", height).Infof("writing to csv")

	err = writer.Write(append([]string{"ID"}, common.AllAssets...)) // Write headers
	if err != nil {
		return err
	}

	// Build the csv
	for i, opr := range block.OPRs {
		// fmt.Println(opr.FactomDigitalID)
		line := []string{fmt.Sprintf("%d", i)}
		for _, asset := range common.AllAssets {
			// line = append(line, fmt.Sprintf("%d", int64(opr.Assets[asset]*1e4)%10000))
			v := opr.Assets[asset]
			// jitter
			// sub := 1.0
			// if rand.Intn(2) == 1 {
			// 	sub = -1.0
			// }
			// jitter := (rand.Float64() / 1000) * sub
			// v = v * jitter
			// fmt.Println(jitter)
			line = append(line, fmt.Sprintf("%.4f", v))
		}

		err = writer.Write(line)
		if err != nil {
			return err
		}
	}

	writer.Flush()
	var _ = file.Close()
	return nil
}

// WriteMinerCSV will write all the miner related stats for a given chain to a csv. This includes
// number of records, difficulties hit, and more.
func (c *ChainRecorder) WriteMinerCSV() error {
	InitLX() // We intend to use the LX hash

	f, err := os.OpenFile(c.filepath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	file := f
	writer := csv.NewWriter(file)

	b := balances.NewBalanceTracker()
	g := NewQuickGrader(c.config, database.NewMapDb(), b)
	err = g.Sync()
	if err != nil {
		return err
	}

	recLog.WithField("blockcount", len(g.GetBlocks())).Infof("writing to csv")
	cutoff, _ := c.config.Int(common.ConfigSubmissionCutOff)
	err = writer.Write([]string{
		"blockheight", "records",
		"1st Diff (int)", "1st Diff (hex)",
		"Last Graded Place", "Last Graded Diff (int)", "Last Graded Diff (hex)",
		"Cutoff", fmt.Sprintf("%d cutoff (int)", cutoff), fmt.Sprintf("%d cutoff (hex)", cutoff),
		"Last OPR Place", "Last OPR (int)", "Last OPR (hex)",
	}) // Write headers
	if err != nil {
		return err
	}

	// Build the csv
	for i, block := range g.GetBlocks() {
		var _ = i
		last := 50
		if len(block.OPRs) < 50 {
			last = len(block.OPRs) - 1
		}
		if last < 0 {
			continue
		}

		cutoffD := CalculateMinimumDifficultyFromOPRs(block.OPRs, cutoff)

		err = writer.Write([]string{
			fmt.Sprintf("%d", block.Dbht),
			fmt.Sprintf("%d", len(block.OPRs)),

			fmt.Sprintf("%d", block.OPRs[0].Difficulty),
			fmt.Sprintf("%x", block.OPRs[0].Difficulty),

			fmt.Sprintf("%d", last),
			fmt.Sprintf("%d", block.OPRs[last].Difficulty),
			fmt.Sprintf("%x", block.OPRs[last].Difficulty),

			fmt.Sprintf("%d", cutoff),
			fmt.Sprintf("%d", cutoffD),
			fmt.Sprintf("%x", cutoffD),

			fmt.Sprintf("%d", len(block.OPRs)),
			fmt.Sprintf("%d", block.OPRs[len(block.OPRs)-1].Difficulty),
			fmt.Sprintf("%x", block.OPRs[len(block.OPRs)-1].Difficulty),
		})
		if err != nil {
			return err
		}
	}

	writer.Flush()
	var _ = file.Close()

	return nil
}
