package chat

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"
)

// Twitch lê o chat de um canal via IRC anônimo (justinfan = leitura sem auth).
type Twitch struct {
	Channel string // nome do canal, minúsculo, sem #
	out     chan Message
}

func NewTwitch(channel string) *Twitch {
	return &Twitch{
		Channel: strings.ToLower(strings.TrimPrefix(channel, "#")),
		out:     make(chan Message, 50),
	}
}

func (t *Twitch) Messages() <-chan Message { return t.out }

// Start conecta no IRC da Twitch e emite mensagens. Reconecta sozinho se cair.
func (t *Twitch) Start() error {
	for {
		if err := t.run(); err != nil {
			log.Printf("[twitch] conexão caiu: %v — reconectando em 5s", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func (t *Twitch) run() error {
	conn, err := net.DialTimeout("tcp", "irc.chat.twitch.tv:6667", 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	nick := fmt.Sprintf("justinfan%d", 10000+rand.Intn(80000))
	fmt.Fprintf(conn, "NICK %s\r\n", nick)
	fmt.Fprintf(conn, "JOIN #%s\r\n", t.Channel)
	log.Printf("[twitch] conectado em #%s (anônimo, somente leitura)", t.Channel)

	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024)
	for sc.Scan() {
		line := sc.Text()

		// keepalive do IRC
		if strings.HasPrefix(line, "PING") {
			fmt.Fprintf(conn, "PONG :tmi.twitch.tv\r\n")
			continue
		}

		// formato: :nick!user@host PRIVMSG #canal :mensagem
		if idx := strings.Index(line, " PRIVMSG #"); idx > 0 {
			nickEnd := strings.Index(line, "!")
			if nickEnd < 1 {
				continue
			}
			user := line[1:nickEnd]
			msgIdx := strings.Index(line[idx:], " :")
			if msgIdx < 0 {
				continue
			}
			text := line[idx+msgIdx+2:]
			select {
			case t.out <- Message{User: user, Text: text, At: time.Now()}:
			default: // fila cheia (raid?), descarta
			}
		}
	}
	return fmt.Errorf("stream encerrou: %v", sc.Err())
}
