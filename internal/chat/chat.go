package chat

import (
	"strings"
	"sync"
	"time"
)

// Message é uma mensagem de chat, de qualquer fonte.
type Message struct {
	User string
	Text string
	At   time.Time
}

// Source é qualquer origem de chat (Twitch real, simulador, etc).
type Source interface {
	// Start conecta e começa a emitir mensagens no channel. Bloqueante; rode em goroutine.
	Start() error
	// Messages é o stream de mensagens recebidas.
	Messages() <-chan Message
}

// Buffer guarda as últimas N mensagens pra virar contexto no prompt.
type Buffer struct {
	mu   sync.Mutex
	msgs []Message
	max  int
}

func NewBuffer(max int) *Buffer {
	return &Buffer{max: max}
}

func (b *Buffer) Add(m Message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, m)
	if len(b.msgs) > b.max {
		b.msgs = b.msgs[len(b.msgs)-b.max:]
	}
}

// Context devolve as mensagens recentes formatadas pro prompt (vazio se não há chat).
func (b *Buffer) Context() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.msgs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Chat recente da live:\n")
	for _, m := range b.msgs {
		sb.WriteString(m.User)
		sb.WriteString(": ")
		sb.WriteString(m.Text)
		sb.WriteString("\n")
	}
	return sb.String()
}
