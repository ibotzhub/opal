package lex

import "testing"

func TestLookup(t *testing.T) {
	type test struct {
		r    rune   // the character to look up
		want byte   // expected result: the char itself if it's a token, 0 if not
	}
	tests := []test{
		{' ', ' '},
		{'=', '='},
		{'{', '{'},
		{'}', '}'},
		{'(', '('},
		{')', ')'},
		{';', ';'},
		{',', ','},
		{'+', '+'},
		{'-', '-'},
		{'|', '|'},
		// non-token chars return 0
		{'a', 0},
		{'z', 0},
		{'0', 0},
	}
	for _, tst := range tests {
		got := lookup(int(tst.r))
		if got != tst.want {
			t.Errorf("lookup(%d) = %q (%d), want %q (%d)",
				int(tst.r), string(got), got, string(tst.want), tst.want)
			continue
		}
		t.Logf("lookup('%s') = '%s'", string(tst.r), string(got))
	}
}

// TestLookupIsTokenConsistency verifies that lookup agrees with singleRuneTokens:
// every char in the map should return itself, nothing else should.
func TestLookupIsTokenConsistency(t *testing.T) {
	for r := rune(0); r < 126; r++ {
		got := lookup(int(r))
		_, inMap := singleRuneTokens[r]
		isSpace := r == ' '

		switch {
		case inMap || isSpace:
			if got != byte(r) {
				t.Errorf("lookup(%d/'%s'): expected %d, got %d (in token map)",
					r, string(r), byte(r), got)
			}
		default:
			if got != 0 {
				t.Errorf("lookup(%d/'%s'): expected 0, got %d (not a token)",
					r, string(r), got)
			}
		}
	}
}
