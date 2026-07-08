package tools

import (
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
