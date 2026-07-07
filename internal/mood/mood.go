package mood

import "sync"

type State struct {
	mu       sync.RWMutex
	current  string
	speaking bool
}

var Valid = map[string]bool{
	"neutra": true, "animada": true, "zoeira": true,
	"entediada": true, "surpresa": true, "provocada": true,
}

func New() *State {
	return &State{current: "neutra"}
}

func (s *State) Set(m string) {
	if !Valid[m] {
		m = "neutra"
	}
	s.mu.Lock()
	s.current = m
	s.mu.Unlock()
}

func (s *State) Get() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *State) SetSpeaking(b bool) {
	s.mu.Lock()
	s.speaking = b
	s.mu.Unlock()
}

func (s *State) IsSpeaking() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.speaking
}
