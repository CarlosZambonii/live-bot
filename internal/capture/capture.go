package capture

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Screenshot captura a tela via cosmic-screenshot e devolve o caminho do PNG.
// O binário imprime o caminho do arquivo salvo no stdout.
func Screenshot() (string, error) {
	dir, err := os.MkdirTemp("", "backseat-shot-*")
	if err != nil {
		return "", err
	}
	out, err := exec.Command("cosmic-screenshot",
		"--interactive=false", "--notify=false", "--save-dir", dir).Output()
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("cosmic-screenshot: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		os.RemoveAll(dir)
		return "", fmt.Errorf("cosmic-screenshot: sem caminho no stdout")
	}
	return path, nil
}

// Cleanup remove o screenshot e o diretório temporário dele.
func Cleanup(path string) {
	if path == "" {
		return
	}
	os.Remove(path)
	os.Remove(strings.TrimSuffix(path, "/"+pathBase(path)))
}

func pathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
