package brain

import "testing"

func TestExtractMood(t *testing.T) {
	cases := []struct {
		in, mood, clean string
	}{
		{"[zoeira] morreu de novo", "zoeira", "morreu de novo"},
		{"[animada] boa jogada!", "animada", "boa jogada!"},
		{"sem prefixo aqui", "neutra", "sem prefixo aqui"},
		{"[invalido] texto", "neutra", "[invalido] texto"},
	}
	for _, c := range cases {
		m, clean := ExtractMood(c.in)
		if m != c.mood || clean != c.clean {
			t.Errorf("ExtractMood(%q) = (%q,%q), quer (%q,%q)", c.in, m, clean, c.mood, c.clean)
		}
	}
}
