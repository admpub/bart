//go:build go1.23

// Copyright (c) 2024 Karl Gaissmaier
// SPDX-License-Identifier: MIT

package bitset

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"testing"
)

func BenchmarkBitSetRankChild(b *testing.B) {
	var bs BitSet
	for range 200 {
		bs = bs.Set(uint(rand.IntN(256)))
	}

	// make unique random numbers
	randsUnique := map[int]bool{}
	for range 20 {
		bit := rand.IntN(500)
		if bit > 255 {
			continue
		}
		randsUnique[bit] = true
	}

	// sort them ascending
	rands := slices.Collect(maps.Keys(randsUnique))
	slices.Sort(rands)

	// benchmark Rank with them
	for _, r := range rands {
		b.Run(fmt.Sprintf("%3d", r), func(b *testing.B) {
			for range b.N {
				_ = bs.Rank0(uint(r))
			}
		})
	}
}

func BenchmarkBitSetRankPrefix(b *testing.B) {
	var bs BitSet
	for range 200 {
		bit := rand.IntN(1_000)
		if bit > 511 {
			continue
		}
		bs = bs.Set(uint(bit))
	}

	// make uniques random numbers
	randsUnique := map[int]bool{}
	for range 20 {
		bit := rand.IntN(1_000)
		if bit > 511 {
			continue
		}
		randsUnique[bit] = true
	}

	// sort them ascending
	rands := slices.Collect(maps.Keys(randsUnique))
	slices.Sort(rands)

	// benchmark Rank with them
	for _, r := range rands {
		b.Run(fmt.Sprintf("%3d", r), func(b *testing.B) {
			for range b.N {
				_ = bs.Rank0(uint(r))
			}
		})
	}
}

func BenchmarkBitSetInPlace(b *testing.B) {
	bs := BitSet([]uint64{})
	cs := BitSet([]uint64{})
	for range 200 {
		bs = bs.Set(uint(rand.IntN(512)))
	}
	for range 200 {
		cs = cs.Set(uint(rand.IntN(512)))
	}

	b.Run("InPlaceIntersection len(b)==len(c)", func(b *testing.B) {
		for range b.N {
			(&bs).InPlaceIntersection(cs)
		}
	})

	bs = BitSet([]uint64{})
	cs = BitSet([]uint64{})
	for range 200 {
		bs = bs.Set(uint(rand.IntN(512)))
	}
	for range 200 {
		cs = cs.Set(uint(rand.IntN(512)))
	}
	b.Run("InPlaceUnion len(b)==len(c)", func(b *testing.B) {
		for range b.N {
			(&bs).InPlaceUnion(cs)
		}
	})

	bs = BitSet([]uint64{})
	cs = BitSet([]uint64{})
	for range 200 {
		bs = bs.Set(uint(rand.IntN(512)))
	}
	for range 200 {
		cs = cs.Set(uint(rand.IntN(256)))
	}
	b.Run("InPlaceIntersection len(b)>len(c)", func(b *testing.B) {
		for range b.N {
			(&bs).InPlaceIntersection(cs)
		}
	})

	bs = BitSet([]uint64{})
	cs = BitSet([]uint64{})
	for range 200 {
		bs = bs.Set(uint(rand.IntN(512)))
	}
	for range 200 {
		cs = cs.Set(uint(rand.IntN(256)))
	}
	b.Run("InPlaceUnion len(b)>len(c)", func(b *testing.B) {
		for range b.N {
			(&bs).InPlaceUnion(cs)
		}
	})
	bs = BitSet([]uint64{})
	cs = BitSet([]uint64{})
	for range 200 {
		bs = bs.Set(uint(rand.IntN(256)))
	}
	for range 200 {
		cs = cs.Set(uint(rand.IntN(512)))
	}
	b.Run("InPlaceIntersection len(b)<len(c)", func(b *testing.B) {
		for range b.N {
			(&bs).InPlaceIntersection(cs)
		}
	})

	bs = BitSet([]uint64{})
	cs = BitSet([]uint64{})
	for range 200 {
		bs = bs.Set(uint(rand.IntN(256)))
	}
	for range 200 {
		cs = cs.Set(uint(rand.IntN(512)))
	}
	b.Run("InPlaceUnion len(b)<len(c)", func(b *testing.B) {
		for range b.N {
			(&bs).InPlaceUnion(cs)
		}
	})
}

func BenchmarkWorstCaseLPM(b *testing.B) {
	pfx := BitSet{}.Set(1).Set(510)
	idx := BitSet{}.Set(511).Set(255).Set(127).Set(63).Set(31).Set(15).Set(7).Set(3).Set(1)

	b.Run("LPM-Top-IntersectionTop", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_, _ = pfx.IntersectionTop(idx)
		}
	})

	b.Run("LPM-Test-IntersectsAny", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			_ = pfx.IntersectsAny(idx)
		}
	})

	b.Run("LPM-Test-Iter", func(b *testing.B) {
		var firstPfx uint

		b.ResetTimer()
		for range b.N {
			firstPfx, _ = pfx.FirstSet()

			for idx := uint(511); idx >= firstPfx; idx >>= 1 {
				if pfx.Test(idx) {
					break
				}
			}
		}
	})
}
