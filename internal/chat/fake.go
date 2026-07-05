package chat

import (
	"math/rand"
	"time"
)

// Fake é um simulador de chat pra desenvolvimento sem viewers.
type Fake struct {
	out  chan Message
	rate time.Duration // intervalo médio entre mensagens
}

func NewFake(rate time.Duration) *Fake {
	return &Fake{out: make(chan Message, 50), rate: rate}
}

func (f *Fake) Messages() <-chan Message { return f.out }

var fakeUsers = []string{"gamerzito77", "xX_sniper_Xx", "mariazinha_br", "clipador", "trollzin", "vovo_gamer", "lurker001"}

var fakeLines = []string{
	"KKKKKKKK",
	"morreu de novo mano",
	"esse mapa é impossível",
	"LUL",
	"joga muito (mentira)",
	"alguém sabe que jogo é esse?",
	"Dora, você acha que ele ganha essa?",
	"primeira vez aqui, canal top",
	"dora fala alguma coisa",
	"gg",
	"que horas acaba a live?",
	"F",
	"esse cara é pior que eu jogando",
	"Dora zoa ele por favor kkkk",
	"qual a config do pc?",
}

// Start emite mensagens aleatórias no ritmo configurado. Bloqueante.
func (f *Fake) Start() error {
	for {
		// jitter: entre 0.5x e 1.5x do rate
		jitter := time.Duration(float64(f.rate) * (0.5 + rand.Float64()))
		time.Sleep(jitter)
		f.out <- Message{
			User: fakeUsers[rand.Intn(len(fakeUsers))],
			Text: fakeLines[rand.Intn(len(fakeLines))],
			At:   time.Now(),
		}
	}
}
