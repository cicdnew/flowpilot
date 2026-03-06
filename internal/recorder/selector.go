package recorder

import (
	"sort"

	"web-automation/internal/models"
)

// RankSelectors orders selector candidates by stability score.
func RankSelectors(candidates []models.SelectorCandidate) []models.SelectorCandidate {
	sorted := make([]models.SelectorCandidate, len(candidates))
	copy(sorted, candidates)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})
	return sorted
}

// BestSelector returns the highest-ranked selector string.
func BestSelector(candidates []models.SelectorCandidate) string {
	if len(candidates) == 0 {
		return ""
	}
	ranked := RankSelectors(candidates)
	return ranked[0].Selector
}
