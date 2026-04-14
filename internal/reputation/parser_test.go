package reputation

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in    string
		want  int // 0 means nil
		match bool
	}{
		// Basic positive
		{"+rep", 1, true},
		{"++rep", 2, true},
		{"+++rep", 3, true},
		{"+rep ахуй", 1, true},
		{"+++rep отлично", 3, true},
		{"+++++rep", 5, true},

		// Basic negative
		{"-rep", -1, true},
		{"--rep", -2, true},
		{"--реп", -2, true},
		{"-реп плохо", -1, true},

		// Cyrillic
		{"+реп", 1, true},
		{"+++реп", 3, true},

		// Case-insensitive
		{"+REP", 1, true},
		{"+Rep", 1, true},
		{"+Реп", 1, true},
		{"+РЕП", 1, true},

		// Punctuation after rep
		{"+rep,", 1, true},
		{"+реп.", 1, true},
		{"+rep!", 1, true},
		{"+rep\n", 1, true},

		// Mixed signs — forbidden
		{"++--rep", 0, false},
		{"+-rep", 0, false},
		{"-+rep", 0, false},

		// Not at the start
		{"ты +rep", 0, false},
		{" +rep", 0, false},
		{"hello +rep world", 0, false},

		// Not a word boundary — letter follows
		{"+repka", 0, false},
		{"+repeated", 0, false},
		{"+репка", 0, false},
		{"+reput", 0, false},

		// No sign
		{"rep", 0, false},
		{"реп", 0, false},

		// Empty / junk
		{"", 0, false},
		{"+", 0, false},
		{"+++", 0, false},
		{"+++ rep", 0, false}, // space between signs and rep
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := parseText(c.in)
			if !c.match {
				if got != nil {
					t.Fatalf("Parse(%q) = %+v, want nil", c.in, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("Parse(%q) = nil, want delta=%d", c.in, c.want)
			}
			if got.Delta != c.want {
				t.Fatalf("Parse(%q).Delta = %d, want %d", c.in, got.Delta, c.want)
			}
		})
	}
}
