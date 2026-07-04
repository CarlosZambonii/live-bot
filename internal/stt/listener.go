package stt

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
)

const (
	sampleRate   = 16000
	frameMs      = 30                              // duração de cada frame analisado
	frameSamples = sampleRate * frameMs / 1000     // 480 samples
	frameBytes   = frameSamples * 2                // S16_LE = 2 bytes/sample

	preRollFrames  = 10  // ~300ms guardados antes da fala começar
	startFrames    = 3   // frames ativos seguidos pra considerar "começou a falar"
	hangoverFrames = 25  // ~750ms de silêncio pra considerar "terminou"
	maxFrames      = 500 // ~15s: corta segmentos longos demais (segurança)
	minFrames      = 15  // ~450ms: descarta segmentos curtos demais (estalos)
)

// Listener: escuta contínua com VAD por energia (RMS).
type Listener struct {
	Threshold float64       // limiar de RMS pra considerar fala (calibrar; ~300-800)
	Segments  chan string   // caminhos de wav prontos pra transcrever
}

func NewListener(threshold float64) *Listener {
	return &Listener{
		Threshold: threshold,
		Segments:  make(chan string, 3),
	}
}

// Start abre o mic em stream contínuo e emite segmentos de fala no channel.
// Roda pra sempre; chame numa goroutine.
func (l *Listener) Start() error {
	cmd := exec.Command("arecord", "-q",
		"-t", "raw", "-f", "S16_LE", "-r", fmt.Sprint(sampleRate), "-c", "1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("arecord stream: %w", err)
	}

	var (
		preRoll  [][]byte // buffer circular do pré-fala
		segment  [][]byte // frames da fala em andamento
		speaking bool
		silence  int
		active   int
	)

	frame := make([]byte, frameBytes)
	for {
		if _, err := io.ReadFull(stdout, frame); err != nil {
			return fmt.Errorf("mic stream caiu: %w", err)
		}
		f := make([]byte, frameBytes)
		copy(f, frame)
		energy := rms(f)

		if !speaking {
			// mantém o pré-roll girando
			preRoll = append(preRoll, f)
			if len(preRoll) > preRollFrames {
				preRoll = preRoll[1:]
			}
			if energy >= l.Threshold {
				active++
				if active >= startFrames {
					speaking = true
					segment = append([][]byte{}, preRoll...)
					silence = 0
				}
			} else {
				active = 0
			}
			continue
		}

		// falando: acumula
		segment = append(segment, f)
		if energy < l.Threshold {
			silence++
		} else {
			silence = 0
		}

		if silence >= hangoverFrames || len(segment) >= maxFrames {
			speaking, active, silence = false, 0, 0
			if len(segment) >= minFrames {
				if path, err := writeWav(segment); err == nil {
					select {
					case l.Segments <- path:
					default:
						os.Remove(path) // fila cheia: descarta o mais novo
						log.Println("[vad] fila cheia, segmento descartado")
					}
				}
			}
			segment = nil
			preRoll = nil
		}
	}
}

// rms calcula a energia média do frame (amostras S16_LE).
func rms(frame []byte) float64 {
	var sum float64
	n := len(frame) / 2
	for i := 0; i < n; i++ {
		s := int16(binary.LittleEndian.Uint16(frame[i*2:]))
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(n))
}

// writeWav grava os frames num arquivo wav (header PCM 16-bit mono 16kHz).
func writeWav(frames [][]byte) (string, error) {
	tmp, err := os.CreateTemp("", "backseat-seg-*.wav")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	var dataLen int
	for _, f := range frames {
		dataLen += len(f)
	}

	h := make([]byte, 44)
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+dataLen))
	copy(h[8:], "WAVE")
	copy(h[12:], "fmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1) // PCM
	binary.LittleEndian.PutUint16(h[22:], 1) // mono
	binary.LittleEndian.PutUint32(h[24:], sampleRate)
	binary.LittleEndian.PutUint32(h[28:], sampleRate*2) // byte rate
	binary.LittleEndian.PutUint16(h[32:], 2)            // block align
	binary.LittleEndian.PutUint16(h[34:], 16)           // bits
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(dataLen))

	if _, err := tmp.Write(h); err != nil {
		return "", err
	}
	for _, f := range frames {
		if _, err := tmp.Write(f); err != nil {
			return "", err
		}
	}
	return tmp.Name(), nil
}
