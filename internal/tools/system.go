package tools

import (
	"math/rand"
	"strings"
	"os"
	"fmt"
	"os/exec"
	"strconv"
)

// SetVolume ajusta o volume do sistema. delta em % (ex: -20 abaixa, +20 sobe), ou abs se absolute=true.
func SetVolume(delta int, absolute bool) string {
	var arg string
	if absolute {
		arg = strconv.Itoa(delta) + "%"
	} else if delta >= 0 {
		arg = "+" + strconv.Itoa(delta) + "%"
	} else {
		arg = strconv.Itoa(delta) + "%"
	}
	cmd := exec.Command("pactl", "set-sink-volume", "@DEFAULT_SINK@", arg)
	if err := cmd.Run(); err != nil {
		return "erro ao ajustar volume: " + err.Error()
	}
	return fmt.Sprintf("volume ajustado (%s)", arg)
}

// MusicControl controla o player de mídia via playerctl. action: play, pause, play-pause, next, previous.
func MusicControl(action string) string {
	valid := map[string]bool{"play": true, "pause": true, "play-pause": true, "next": true, "previous": true}
	if !valid[action] {
		return "ação de música inválida: " + action
	}
	cmd := exec.Command("playerctl", action)
	if err := cmd.Run(); err != nil {
		return "erro no player (nenhuma música tocando?): " + err.Error()
	}
	if action == "next" || action == "previous" {
		out, _ := exec.Command("playerctl", "metadata", "--format", "{{ artist }} - {{ title }}").Output()
		return "ok, agora tocando: " + string(out)
	}
	return "música: " + action
}


// ListSkins lê os .vrm disponíveis na pasta skins_disk.
func ListSkins() []string {
	entries, err := os.ReadDir("skins_disk")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".vrm") {
			names = append(names, strings.TrimSuffix(e.Name(), ".vrm"))
		}
	}
	return names
}

// PickSkin resolve o pedido: vazio/"aleatoria"=random, número=índice, nome=match. Devolve o nome escolhido ou "".
func PickSkin(pedido string) string {
	skins := ListSkins()
	if len(skins) == 0 {
		return ""
	}
	p := strings.ToLower(strings.TrimSpace(pedido))
	if p == "" || strings.Contains(p, "aleat") || strings.Contains(p, "qualquer") || strings.Contains(p, "random") {
		return skins[rand.Intn(len(skins))]
	}
	// número (skin 5)
	if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(p, "skin"))); err == nil {
		if n >= 1 && n <= len(skins) {
			return skins[n-1]
		}
	}
	// nome parcial
	for _, s := range skins {
		if strings.Contains(strings.ToLower(s), p) {
			return s
		}
	}
	return skins[rand.Intn(len(skins))] // não achou: aleatória
}