package mood

import (
	"sync"
	"time"
)

type State struct {
	mu       sync.RWMutex
	current  string
	speaking bool
	dancing bool
	anim string
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
func (s *State) SetDancing(b bool) {
	s.mu.Lock()
	s.dancing = b
	s.mu.Unlock()
}

func (s *State) IsDancing() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dancing
}
func (s *State) SetAnim(a string) {
	s.mu.Lock()
	s.anim = a
	s.mu.Unlock()
	// auto-limpa depois de 3s (a animação toca uma vez e volta ao idle)
	go func() {
		time.Sleep(3 * time.Second)
		s.mu.Lock()
		if s.anim == a {
			s.anim = ""
		}
		s.mu.Unlock()
	}()
}

func (s *State) Anim() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.anim
}
