package graderDelegateStake

import "sort"

// baseGradedBlockV2 is an spr set that has been graded
type baseGradedBlockV2 struct {
	sprs   []*GradingDelegatedSPR
	cutoff int
	height int32
	count  int

	shorthashes []string
}

func (b *baseGradedBlockV2) cloneSPRS(sprs []*GradingDelegatedSPR) {
	b.sprs = nil
	for _, o := range sprs {
		b.sprs = append(b.sprs, o.Clone())
	}
	b.count = len(sprs)
}

func (b *baseGradedBlockV2) Count() int {
	return b.count
}

// AmountToGrade returns the number of SPRs the grading algorithm attempted to use in the process.
func (b *baseGradedBlockV2) AmountGraded() int {
	return len(b.sprs)
}

func (b *baseGradedBlockV2) createShortHashes(count int) {
	shortHashes := make([]string, count)
	if len(b.sprs) >= count {
		for i := 0; i < count; i++ {
			shortHashes[i] = b.sprs[i].Shorthash()
		}
	}
	b.shorthashes = shortHashes
}

// Graded returns the SPRs that made it into the cutoff
func (b *baseGradedBlockV2) Graded() []*GradingDelegatedSPR {
	return b.sprs
}

// filter out duplicate GradingSPRs. an SPR is a duplicate when both
// nonce and sprhash are the same
func (b *baseGradedBlockV2) filterDuplicates() {
	filtered := make([]*GradingDelegatedSPR, 0)

	added := make(map[string]bool)
	for _, v := range b.sprs {
		id := v.CoinbaseAddress
		if !added[id] {
			filtered = append(filtered, v)
			added[id] = true
		}
	}

	b.sprs = filtered
}

func (b *baseGradedBlockV2) sortByBalance(limit int) {
	sort.SliceStable(b.sprs, func(i, j int) bool {
		return b.sprs[i].balanceOfPEG > b.sprs[j].balanceOfPEG
	})
	if limit > len(b.sprs) {
		limit = len(b.sprs)
	}
	b.sprs = b.sprs[:limit]
}

func (b *baseGradedBlockV2) Cutoff() int { return b.cutoff }
