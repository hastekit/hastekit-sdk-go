package utils

import (
	"log/slog"
	"math/rand"
)

func WeightedRandomIndex(weights []int) int {
	n := len(weights)

	total := 0
	for _, w := range weights {
		if w < 0 {
			slog.Warn("weights must be non-negative")
			continue
		}
		total += w
	}

	if total == 0 {
		return rand.Intn(n)
	}

	r := rand.Intn(total) // [0, total)
	upto := 0

	for i, w := range weights {
		upto += w
		if r < upto {
			return i
		}
	}

	return -1 // should never happen
}
