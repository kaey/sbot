// Copyright 2016 Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is copied from https://talks.golang.org/2012/chat.slide
// and slightly modified for this bot needs.

package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"strings"
)

// Prefix is a Markov chain prefix of one or more words.
type Prefix []string

// String returns the Prefix as a string (for use as a map key).
func (p Prefix) String() string {
	return strings.Join(p, " ")
}

// Shift removes the first word from the Prefix and appends the given word.
func (p Prefix) Shift(word string) string {
	elem := p[0]
	copy(p, p[1:])
	p[len(p)-1] = word
	return elem
}

// LeftShift removes the last word from the Prefix and prepends the given word.
func (p Prefix) LeftShift(word string) string {
	elem := p[len(p)-1]
	copy(p[1:], p[:len(p)-1])
	p[0] = word
	return elem
}

// Chain contains a map ("chain") of prefixes to a list of suffixes.
// A prefix is a string of prefixLen words joined with spaces.
// A suffix is a single word. A prefix can have multiple suffixes.
type Chain struct {
	prefixChain map[string][]string
	suffixChain map[string][]string
	prefixLen   int
}

// NewChain returns a new Chain with prefixes of prefixLen words.
func NewChain(prefixLen int) *Chain {
	return &Chain{make(map[string][]string), make(map[string][]string), prefixLen}
}

// Build reads text from the provided Reader and
// parses it into prefixes and suffixes that are stored in Chain.
func (c *Chain) Build(r io.Reader) {
	br := bufio.NewReader(r)
	p := make(Prefix, c.prefixLen)
	for {
		var word string
		if _, err := fmt.Fscan(br, &word); err != nil {
			break
		}
		key := p.String()
		c.prefixChain[key] = append(c.prefixChain[key], word)
		prevWord := p.Shift(word)
		prevKey := p.String()
		c.suffixChain[prevKey] = append(c.suffixChain[prevKey], prevWord)
	}
}

// Generate returns a string of at most n words generated from Chain.
func (c *Chain) Generate(n int) string {
	p := make(Prefix, c.prefixLen)
	var words []string
	for i := 0; i < n; i++ {
		choices := c.prefixChain[p.String()]
		if len(choices) == 0 {
			break
		}
		next := choices[rand.Intn(len(choices))]
		words = append(words, next)
		p.Shift(next)
	}
	return strings.Join(words, " ")
}

// GenerateWithKeyword returns a string, generated from Chain, containing keyword.
func (c *Chain) GenerateWithKeyword(keyword string, n int) string {
	var p Prefix
	for prefix := range c.prefixChain {
		if strings.Contains(strings.ToLower(prefix), strings.ToLower(keyword)) {
			p = strings.Split(prefix, " ")
		}
	}

	if p == nil {
		return ""
	}

	words := make([]string, c.prefixLen)
	copy(words, p)
	np := make(Prefix, c.prefixLen)
	copy(np, p)
	for i := 0; i < n; i++ {
		choices := c.prefixChain[np.String()]
		if len(choices) == 0 {
			break
		}
		next := choices[rand.Intn(len(choices))]
		words = append(words, next)
		np.Shift(next)
	}

	copy(np, p)
	for i := 0; i < n; i++ {
		choices := c.suffixChain[np.String()]
		if len(choices) == 0 {
			break
		}
		prev := choices[rand.Intn(len(choices))]
		words = append([]string{prev}, words...)
		np.LeftShift(prev)
	}
	return strings.TrimSpace(strings.Join(words, " "))
}
