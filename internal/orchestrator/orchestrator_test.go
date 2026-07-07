package orchestrator

import "testing"

func TestIsRealSpeech(t *testing.T) {
	cases := map[string]bool{
		"":                     false,
		"oi":                   false,
		"12345":                false,
		"e aí dora tudo certo": true,
		"aaa aaa aaa aaa aaa aaa aaa aaa": false, // repetitivo
	}
	for in, want := range cases {
		if got := isRealSpeech(in); got != want {
			t.Errorf("isRealSpeech(%q) = %v, quer %v", in, got, want)
		}
	}
}

func TestIsLooping(t *testing.T) {
	loop := "que é o que é o que é o que é o que é o que é o que é o que é"
	if !isLooping(loop) {
		t.Errorf("deveria detectar loop: %q", loop)
	}
	normal := "hoje eu joguei bem demais nessa partida contra o boss final"
	if isLooping(normal) {
		t.Errorf("falso positivo de loop: %q", normal)
	}
}

func TestNeedsSearch(t *testing.T) {
	yes := []string{"qual o patch atual", "último campeão adicionado", "que dia é hoje"}
	no := []string{"você tá bem", "boa jogada", "vamos nessa"}
	for _, s := range yes {
		if !needsSearch(s) {
			t.Errorf("needsSearch(%q) devia ser true", s)
		}
	}
	for _, s := range no {
		if needsSearch(s) {
			t.Errorf("needsSearch(%q) devia ser false", s)
		}
	}
}

func TestAnimForMood(t *testing.T) {
	if animForMood("surpresa") != "Surprised" {
		t.Error("surpresa deveria mapear Surprised")
	}
	if animForMood("neutra") != "" {
		t.Error("neutra não deveria ter animação")
	}
}
