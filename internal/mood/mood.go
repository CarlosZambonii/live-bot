package mood

import (
	"sync"
	"time"
)

type State struct {
	mu       sync.RWMutex
	current  string
	target   string
	targetHits int
	speaking bool
	dancing bool
	anim string
	mouth float64
	lastSaid string
	object string
	skin   string
	energy float64
}

var Valid = map[string]bool{
	"neutra": true, "animada": true, "zoeira": true,
	"entediada": true, "surpresa": true, "provocada": true,
}

func New() *State {
	return &State{current: "neutra", energy: 1.0}
}

func (s *State) Set(m string) {
	if !Valid[m] {
		m = "neutra"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if m == s.current {
		s.target = m
		s.targetHits = 0
		return
	}
	// inércia: humor novo precisa de confirmações pra virar atual
	if m == s.target {
		s.targetHits++
	} else {
		s.target = m
		s.targetHits = 1
	}
	// estados intensos resistem mais a mudar (custam +1 confirmação pra sair)
	needed := 2
	if s.current == "provocada" || s.current == "animada" {
		needed = 3
	}
	// pra neutra é mais fácil (decai natural)
	if m == "neutra" {
		needed = 2
	}
	if s.targetHits >= needed {
		s.current = m
		s.targetHits = 0
	}
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
func (s *State) SetMouth(v float64) {
	s.mu.Lock()
	s.mouth = v
	s.mu.Unlock()
}

func (s *State) Mouth() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mouth
}


func (s *State) SetLastSaid(t string) {
	s.mu.Lock()
	s.lastSaid = t
	s.mu.Unlock()
}

func (s *State) LastSaid() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSaid
}
func (s *State) SetObject(o string) {
	s.mu.Lock()
	s.object = o
	s.mu.Unlock()
}

func (s *State) Object() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.object
}


func (s *State) SetSkin(sk string) {
	s.mu.Lock()
	s.skin = sk
	s.mu.Unlock()
}

func (s *State) Skin() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.skin == "" {
		return "avatar"
	}
	return s.skin
}
func (s *State) SetEnergy(e float64) {
	s.mu.Lock()
	if e < 0 {
		e = 0
	} else if e > 1 {
		e = 1
	}
	s.energy = e
	s.mu.Unlock()
}

func (s *State) Energy() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.energy
}

// EnergyPhase traduz a energia numa fase (pro comportamento).
func (s *State) EnergyPhase() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	switch {
	case s.energy > 0.7:
		return "cheia"
	case s.energy > 0.4:
		return "normal"
	case s.energy > 0.15:
		return "cansada"
	default:
		return "sonolenta"
	}
}


// SetForce muda o humor imediatamente, ignorando a inércia (pra ações explícitas: recompensas, comandos).
func (s *State) SetForce(m string) {
	if !Valid[m] {
		m = "neutra"
	}
	s.mu.Lock()
	s.current = m
	s.target = m
	s.targetHits = 0
	s.mu.Unlock()
}