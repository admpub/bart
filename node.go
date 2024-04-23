// Copyright (c) 2024 Karl Gaissmaier
// SPDX-License-Identifier: MIT

package bart

import (
	"cmp"
	"net/netip"
	"slices"

	"github.com/bits-and-blooms/bitset"
)

const (
	strideLen       = 8                    // octet
	maxTreeDepth    = 128 / strideLen      // 16
	maxNodeChildren = 1 << strideLen       // 256
	maxNodePrefixes = 1 << (strideLen + 1) // 512
)

// node is a level node in the multibit-trie.
// A node has prefixes and children.
//
// The prefixes form a complete binary tree, see the artlookup.pdf
// paper in the doc folder to understand the data structure.
//
// In contrast to the ART algorithm, popcount-compressed slices are used
// instead of fixed-size arrays.
//
// The array slots are also not pre-allocated as in the ART algorithm,
// but backtracking is used for the longest-prefix-match.
//
// The lookup is then slower by a factor of about 2, but this is
// the intended trade-off to prevent memory consumption from exploding.
type node[V any] struct {
	prefixesBitset *bitset.BitSet
	childrenBitset *bitset.BitSet

	// popcount compressed slices
	prefixes []V
	children []*node[V]
}

// newNode, BitSets have to be initialized.
func newNode[V any]() *node[V] {
	return &node[V]{
		prefixesBitset: bitset.New(0), // init BitSet
		childrenBitset: bitset.New(0), // init BitSet
	}
}

// isEmpty returns true if node has neither prefixes nor children.
func (n *node[V]) isEmpty() bool {
	return len(n.prefixes) == 0 && len(n.children) == 0
}

// ################## prefixes ################################

// prefixRank, Rank() is the key of the popcount compression algorithm,
// mapping between bitset index and slice index.
func (n *node[V]) prefixRank(baseIdx uint) int {
	// adjust offset by one to slice index
	return int(n.prefixesBitset.Rank(baseIdx)) - 1
}

// insertPrefix adds the route octet/prefixLen, with value val.
// Just an adapter for insertIdx.
func (n *node[V]) insertPrefix(octet byte, prefixLen int, val V) {
	n.insertIdx(prefixToBaseIndex(octet, prefixLen), val)
}

// insertIdx adds the route for baseIdx, with value val.
func (n *node[V]) insertIdx(baseIdx uint, val V) {
	// prefix exists, overwrite val
	if n.prefixesBitset.Test(baseIdx) {
		n.prefixes[n.prefixRank(baseIdx)] = val
		return
	}

	// new, insert into bitset and slice
	n.prefixesBitset.Set(baseIdx)
	n.prefixes = slices.Insert(n.prefixes, n.prefixRank(baseIdx), val)
}

// deletePrefix removes the route octet/prefixLen. Reports whether the
// prefix existed in the table prior to deletion.
func (n *node[V]) deletePrefix(octet byte, prefixLen int) (wasPresent bool) {
	baseIdx := prefixToBaseIndex(octet, prefixLen)

	// no route entry
	if !n.prefixesBitset.Test(baseIdx) {
		return false
	}

	rnk := n.prefixRank(baseIdx)

	// delete from slice
	n.prefixes = slices.Delete(n.prefixes, rnk, rnk+1)

	// delete from bitset, followed by Compact to reduce memory consumption
	n.prefixesBitset.Clear(baseIdx)
	n.prefixesBitset.Compact()

	return true
}

// updatePrefix, update or set the value at prefix via callback.
func (n *node[V]) updatePrefix(octet byte, prefixLen int, cb func(V, bool) V) (val V) {
	// calculate idx once
	baseIdx := prefixToBaseIndex(octet, prefixLen)

	var ok bool
	var rnk int

	// if prefix is set, get current value
	if ok = n.prefixesBitset.Test(baseIdx); ok {
		rnk = n.prefixRank(baseIdx)
		val = n.prefixes[rnk]
	}

	// callback function to get updated or new value
	val = cb(val, ok)

	// prefix is already set, update and return value
	if ok {
		n.prefixes[rnk] = val
		return val
	}

	// new prefix, insert into bitset ...
	n.prefixesBitset.Set(baseIdx)

	// bitset has changed, recalc rank
	rnk = n.prefixRank(baseIdx)

	// ... and insert value into slice
	n.prefixes = slices.Insert(n.prefixes, rnk, val)

	return val
}

// lpmByIndex does a route lookup for idx in the 8-bit (stride) routing table
// at this depth and returns (baseIdx, value, true) if a matching
// longest prefix exists, or ok=false otherwise.
//
// backtracking is fast, it's just a bitset test and, if found, one popcount.
func (n *node[V]) lpmByIndex(idx uint) (baseIdx uint, val V, ok bool) {
	// max steps in backtracking is the stride length.
	for {
		if n.prefixesBitset.Test(idx) {
			// longest prefix match
			return idx, n.prefixes[n.prefixRank(idx)], true
		}

		if idx == 0 {
			break
		}

		// cache friendly backtracking to the next less specific route.
		// thanks to the complete binary tree it's just a shift operation.
		idx >>= 1
	}

	// not found (on this level)
	return 0, val, false
}

// lpmByOctet is an adapter to lpmByIndex.
func (n *node[V]) lpmByOctet(octet byte) (baseIdx uint, val V, ok bool) {
	return n.lpmByIndex(octetToBaseIndex(octet))
}

// lpmByPrefix is an adapter to lpmByIndex.
func (n *node[V]) lpmByPrefix(octet byte, bits int) (baseIdx uint, val V, ok bool) {
	return n.lpmByIndex(prefixToBaseIndex(octet, bits))
}

// getValByIndex for baseIdx.
func (n *node[V]) getValByIndex(baseIdx uint) (val V, ok bool) {
	if n.prefixesBitset.Test(baseIdx) {
		return n.prefixes[n.prefixRank(baseIdx)], true
	}
	return
}

// getValByPrefix, adapter for getValByIndex.
func (n *node[V]) getValByPrefix(octet byte, bits int) (val V, ok bool) {
	return n.getValByIndex(prefixToBaseIndex(octet, bits))
}

// apmByPrefix does an all prefix match in the 8-bit (stride) routing table
// at this depth and returns all matching baseIdx's.
func (n *node[V]) apmByPrefix(octet byte, bits int) (result []uint) {
	// skip intermediate nodes
	if len(n.prefixes) == 0 {
		return
	}

	idx := prefixToBaseIndex(octet, bits)
	for {
		if n.prefixesBitset.Test(idx) {
			result = append(result, idx)
		}

		if idx == 0 {
			break
		}

		// cache friendly backtracking to the next less specific route.
		// thanks to the complete binary tree it's just a shift operation.
		idx >>= 1
	}

	// sort in ascending order
	slices.Sort(result)
	return result
}

// allStrideIndexes returns all baseIndexes set in this stride node in ascending order.
func (n *node[V]) allStrideIndexes() []uint {
	all := make([]uint, 0, maxNodePrefixes)
	_, all = n.prefixesBitset.NextSetMany(0, all)
	return all
}

// ################## children ################################

// childRank, Rank() is the key of the popcount compression algorithm,
// mapping between bitset index and slice index.
func (n *node[V]) childRank(octet byte) int {
	// adjust offset by one to slice index
	return int(n.childrenBitset.Rank(uint(octet))) - 1
}

// insertChild, insert the child
func (n *node[V]) insertChild(octet byte, child *node[V]) {
	// child exists, overwrite it
	if n.childrenBitset.Test(uint(octet)) {
		n.children[n.childRank(octet)] = child
		return
	}

	// new insert into bitset and slice
	n.childrenBitset.Set(uint(octet))
	n.children = slices.Insert(n.children, n.childRank(octet), child)
}

// deleteChild, delete the child at octet. It is valid to delete a non-existent child.
func (n *node[V]) deleteChild(octet byte) {
	if !n.childrenBitset.Test(uint(octet)) {
		return
	}

	rnk := n.childRank(octet)

	// delete from slice
	n.children = slices.Delete(n.children, rnk, rnk+1)

	// delete from bitset, followed by Compact to reduce memory consumption
	n.childrenBitset.Clear(uint(octet))
	n.childrenBitset.Compact()
}

// getChild returns the child pointer for octet, or nil if none.
func (n *node[V]) getChild(octet byte) *node[V] {
	if !n.childrenBitset.Test(uint(octet)) {
		return nil
	}

	return n.children[n.childRank(octet)]
}

// allChildAddrs returns the octets of all child nodes in ascending order.
func (n *node[V]) allChildAddrs() []uint {
	all := make([]uint, maxNodeChildren)
	_, all = n.childrenBitset.NextSetMany(0, all)
	return all
}

// #################### nodes #############################################

// overlapsRec returns true if any IP in the nodes n or o overlaps.
// First test the routes, then the children and if no match rec-descent
// for child nodes with same octet.
func (n *node[V]) overlapsRec(o *node[V]) bool {
	// dynamically allot the host routes from prefixes
	nAllotIndex := [maxNodePrefixes]bool{}
	oAllotIndex := [maxNodePrefixes]bool{}

	// 1. test if any routes overlaps?

	nOk := len(n.prefixes) > 0
	oOk := len(o.prefixes) > 0
	var nIdx, oIdx uint
	// zig-zag, for all routes in both nodes ...
	for {
		if nOk {
			// range over bitset, node n
			if nIdx, nOk = n.prefixesBitset.NextSet(nIdx); nOk {
				// get range of host routes for this prefix
				lowerBound, upperBound := lowerUpperBound(nIdx)

				// insert host routes (octet/8) for this prefix,
				// some sort of allotment
				for i := lowerBound; i <= upperBound; i++ {
					// zig-zag, fast return
					if oAllotIndex[i] {
						return true
					}
					nAllotIndex[i] = true
				}
				nIdx++
			}
		}

		if oOk {
			// range over bitset, node o
			if oIdx, oOk = o.prefixesBitset.NextSet(oIdx); oOk {
				// get range of host routes for this prefix
				lowerBound, upperBound := lowerUpperBound(oIdx)

				// insert host routes (octet/8) for this prefix,
				// some sort of allotment
				for i := lowerBound; i <= upperBound; i++ {
					// zig-zag, fast return
					if nAllotIndex[i] {
						return true
					}
					oAllotIndex[i] = true
				}
				oIdx++
			}
		}
		if !nOk && !oOk {
			break
		}
	}

	// full run, zig-zag didn't already match
	if len(n.prefixes) > 0 && len(o.prefixes) > 0 {
		for i := firstHostIndex; i <= lastHostIndex; i++ {
			if nAllotIndex[i] && oAllotIndex[i] {
				return true
			}
		}
	}

	// 2. test if routes overlaps any child

	nOctets := [maxNodeChildren]bool{}
	oOctets := [maxNodeChildren]bool{}

	nOk = len(n.children) > 0
	oOk = len(o.children) > 0
	var nOctet, oOctet uint
	// zig-zag, for all octets in both nodes ...
	for {
		// range over bitset, node n
		if nOk {
			if nOctet, nOk = n.childrenBitset.NextSet(nOctet); nOk {
				if oAllotIndex[nOctet+firstHostIndex] {
					return true
				}
				nOctets[nOctet] = true
				nOctet++
			}
		}

		// range over bitset, node o
		if oOk {
			if oOctet, oOk = o.childrenBitset.NextSet(oOctet); oOk {
				if nAllotIndex[oOctet+firstHostIndex] {
					return true
				}
				oOctets[oOctet] = true
				oOctet++
			}
		}

		if !nOk && !oOk {
			break
		}
	}

	// 3. rec-descent call for childs with same octet

	if len(n.children) > 0 && len(o.children) > 0 {
		for i := 0; i < len(nOctets); i++ {
			if nOctets[i] && oOctets[i] {
				// get next child node for this octet
				nc := n.getChild(byte(i))
				oc := o.getChild(byte(i))

				// rec-descent
				if nc.overlapsRec(oc) {
					return true
				}
			}
		}
	}

	return false
}

// overlapsPrefix returns true if node overlaps with prefix.
func (n *node[V]) overlapsPrefix(octet byte, pfxLen int) bool {
	// ##################################################
	// 1. test if any route in this node overlaps prefix?

	pfxIdx := prefixToBaseIndex(octet, pfxLen)
	if _, _, ok := n.lpmByIndex(pfxIdx); ok {
		return true
	}

	// #################################################
	// 2. test if prefix overlaps any route in this node

	// lower/upper boundary for octet/pfxLen host routes
	pfxLowerBound := uint(octet) + firstHostIndex
	pfxUpperBound := lastHostIndexOfPrefix(octet, pfxLen)

	// increment to 'next' routeIdx for start in bitset search
	// since pfxIdx already testet by lpm in other direction
	routeIdx := pfxIdx << 1
	var ok bool
	for {
		if routeIdx, ok = n.prefixesBitset.NextSet(routeIdx); !ok {
			break
		}

		routeLowerBound, routeUpperBound := lowerUpperBound(routeIdx)
		if routeLowerBound >= pfxLowerBound && routeUpperBound <= pfxUpperBound {
			return true
		}

		// next route
		routeIdx++
	}

	// #################################################
	// 3. test if prefix overlaps any child in this node

	// set start octet in bitset search with prefix octet
	childOctet := uint(octet)
	for {
		if childOctet, ok = n.childrenBitset.NextSet(childOctet); !ok {
			break
		}

		childIdx := childOctet + firstHostIndex
		if childIdx >= pfxLowerBound && childIdx <= pfxUpperBound {
			return true
		}

		// next round
		childOctet++
	}

	return false
}

// unionRec combines two nodes, changing the receiver node.
// If there are duplicate entries, the value is taken from the other node.
func (n *node[V]) unionRec(o *node[V]) {
	var oIdx uint
	var oOk bool
	// for all prefixes in other node do ...
	for {
		if oIdx, oOk = o.prefixesBitset.NextSet(oIdx); !oOk {
			break
		}
		oVal, _ := o.getValByIndex(oIdx)
		// insert/overwrite prefix/value from oNode to nNode
		n.insertIdx(oIdx, oVal)
		oIdx++
	}

	var oOctet uint
	// for all children in other node do ...
	for {
		if oOctet, oOk = o.childrenBitset.NextSet(oOctet); !oOk {
			break
		}
		oNode := o.getChild(byte(oOctet))

		// get nNode with same octet
		nNode := n.getChild(byte(oOctet))
		if nNode == nil {
			// union child from oNode into nNode
			n.insertChild(byte(oOctet), oNode.cloneRec())
		} else {
			// both nodes have child with octet, call union rec-descent
			nNode.unionRec(oNode)
		}
		oOctet++
	}
}

func (n *node[V]) cloneRec() *node[V] {
	c := newNode[V]()
	if n.isEmpty() {
		return c
	}

	c.prefixesBitset = n.prefixesBitset.Clone() // deep
	c.prefixes = slices.Clone(n.prefixes)       // shallow values

	c.childrenBitset = n.childrenBitset.Clone() // deep
	c.children = slices.Clone(n.children)       // shallow

	// now clone the children deep
	for i, child := range c.children {
		c.children[i] = child.cloneRec()
	}

	return c
}

// walkRec runs recursive the trie, starting at node and
// the cb function is called for each route entry with prefix and value.
// If the cb function returns an error the walk ends prematurely and the
// error is propagated.
func (n *node[V]) walkRec(path []byte, is4 bool, cb func(netip.Prefix, V) error) error {
	// for all prefixes in this node do ...
	for _, idx := range n.allStrideIndexes() {
		val, _ := n.getValByIndex(idx)
		pfx := cidrFromPath(path, idx, is4)

		// make the callback for this prefix
		if err := cb(pfx, val); err != nil {
			// premature end of recursion
			return err
		}
	}

	// for all children in this node do ...
	for _, addr := range n.allChildAddrs() {
		octet := byte(addr)
		path := append(slices.Clone(path), octet)
		child := n.getChild(octet)

		if err := child.walkRec(path, is4, cb); err != nil {
			// premature end of recursion
			return err
		}
	}

	return nil
}

// subnets returns all CIDRs covered by parent pfx.
func (n *node[V]) subnets(path []byte, parentIdx uint, is4 bool) (result []netip.Prefix) {
	// for all routes in this node do ...
	for _, idx := range n.allStrideIndexes() {
		// is this route covered by pfx?
		for i := idx; i >= parentIdx; i >>= 1 {
			if i == parentIdx { // match
				// get CIDR back for idx
				pfx := cidrFromPath(path, idx, is4)

				result = append(result, pfx)
				break
			}
		}
	}

	// for all children in this node do ...
	for _, addr := range n.allChildAddrs() {
		octet := byte(addr)
		idx := octetToBaseIndex(octet)

		// is this child covered by pfx?
		for i := idx; i >= parentIdx; i >>= 1 {
			if i == parentIdx { // match
				// get child for octet
				c := n.getChild(octet)

				// append octet to path
				path := append(slices.Clone(path), octet)

				// all cidrs under this child are covered by pfx
				_ = c.walkRec(path, is4, func(pfx netip.Prefix, _ V) (err error) {
					result = append(result, pfx)
					return
				})
			}
		}
	}

	return result
}

// cmpPrefix, all cidrs are normalized
func cmpPrefix(a, b netip.Prefix) int {
	if cmp := a.Addr().Compare(b.Addr()); cmp != 0 {
		return cmp
	}
	return cmp.Compare(a.Bits(), b.Bits())
}
