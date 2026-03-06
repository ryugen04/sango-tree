package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Tab      key.Binding
	Start    key.Binding
	Stop     key.Binding
	Restart  key.Binding
	StartAll key.Binding
	StopAll  key.Binding
	Logs     key.Binding
	Refresh  key.Binding
	Quit     key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
	),
	Start: key.NewBinding(
		key.WithKeys("u"),
	),
	Stop: key.NewBinding(
		key.WithKeys("d"),
	),
	Restart: key.NewBinding(
		key.WithKeys("r"),
	),
	StartAll: key.NewBinding(
		key.WithKeys("U"),
	),
	StopAll: key.NewBinding(
		key.WithKeys("D"),
	),
	Logs: key.NewBinding(
		key.WithKeys("l"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("R"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
	),
}
