package tui

import (
	"strings"
)

// levenshtein calculates the Levenshtein distance between two strings
func levenshtein(s1, s2 string) int {
	s1 = strings.ToLower(s1)
	s2 = strings.ToLower(s2)
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)

	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}

	matrix := make([][]int, n+1)
	for i := range matrix {
		matrix[i] = make([]int, m+1)
	}

	for i := 0; i <= n; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= m; j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}
	return matrix[n][m]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// findClosestCommand finds the closest command from a list
func findClosestCommand(input string, commands []string) string {
	bestMatch := ""
	minDist := 100 // Arbitrary high number

	// Threshold: allow up to 30% length error or max 3 chars
	threshold := len(input) / 3
	if threshold < 2 {
		threshold = 2
	}

	for _, cmd := range commands {
		dist := levenshtein(input, cmd)
		if dist < minDist {
			minDist = dist
			bestMatch = cmd
		}
	}

	if minDist <= threshold {
		return bestMatch
	}
	return ""
}
