package memories

import (
	"hash/fnv"
	"math"
	"sort"

	"huepattl.de/unterlumen/internal/library"
)

// poolShare is the part of the photos, rarest first, that rediscovered
// photos come from.
const poolShare = 0.05

// rediscovered is five of the rarest photos of any time, the day's turn
// through them, leaving out what the other memories show.
func (w *world) rediscovered(shown map[string]bool) (memory, bool) {
	pool := w.pool()
	if len(pool) == 0 {
		return memory{}, false
	}
	turn := append([]library.MemoryPhoto{}, pool...)
	sort.SliceStable(turn, func(i, j int) bool { return poolOrder(turn[i]) < poolOrder(turn[j]) })
	start := int((w.day.Unix() / 86400 * picksPer) % int64(len(turn)))
	var picks []library.MemoryPhoto
	for i := 0; i < len(turn) && len(picks) < picksPer; i++ {
		if p := turn[(start+i)%len(turn)]; !shown[p.ID] {
			picks = append(picks, p)
		}
	}
	if len(picks) == 0 {
		return memory{}, false
	}
	sort.SliceStable(picks, func(i, j int) bool { return w.rarity.Of(picks[i]) > w.rarity.Of(picks[j]) })
	return memory{card: Card{Kind: "rediscovered", Key: w.today()}, photos: pool, picks: picks}, true
}

// pool is the rarest photos, rarest first: the rarest 5 %, at least five,
// and only photos that are rare at all.
func (w *world) pool() []library.MemoryPhoto {
	type rated struct {
		photo  library.MemoryPhoto
		rarity float64
	}
	var rare []rated
	for _, p := range w.all {
		if r := w.rarity.Of(p); r > 0 {
			rare = append(rare, rated{p, r})
		}
	}
	sort.Slice(rare, func(i, j int) bool {
		return rare[i].rarity > rare[j].rarity || (rare[i].rarity == rare[j].rarity && rare[i].photo.ID < rare[j].photo.ID)
	})
	n := min(len(rare), max(picksPer, int(math.Ceil(poolShare*float64(len(w.all))))))
	out := make([]library.MemoryPhoto, n)
	for i := range out {
		out[i] = rare[i].photo
	}
	return out
}

// poolOrder is a photo's fixed place in the turn through the pool.
func poolOrder(p library.MemoryPhoto) uint64 {
	h := fnv.New64a()
	h.Write([]byte("pool|" + p.ID))
	return h.Sum64()
}
