package main

import (
	"errors"
	"strconv"
	"strings"
)

var (
	errPersonaNameRequired = errors.New("persona name is required")
	errInvalidPersonaID    = errors.New("invalid persona id")
	errPersonaNotFound     = errors.New("persona not found")
)

const settingActivePersonaID = "active_persona_id"

// personaPromptFromDB returns the instruction text for this request. The prompt always comes from
// SQLite (personas.prompt). selectedPersonaID is which persona the client chose for this send/regenerate;
// it must not be the prompt text itself.
// Rules: 0 personas → ""; 1 persona → that row's prompt (selection N/A); 2+ → look up by ID, "" id → no prefix.
func (a *App) personaPromptFromDB(selectedPersonaID string) string {
	if a.store == nil {
		return ""
	}
	personas, err := a.store.GetPersonas()
	if err != nil || len(personas) == 0 {
		return ""
	}
	if len(personas) == 1 {
		return strings.TrimSpace(personas[0].Prompt)
	}
	idStr := strings.TrimSpace(selectedPersonaID)
	if idStr == "" {
		return ""
	}
	pid, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return ""
	}
	for _, p := range personas {
		if p.ID == pid {
			return strings.TrimSpace(p.Prompt)
		}
	}
	return ""
}

// GetPersonas returns all personas (empty if DB unavailable).
func (a *App) GetPersonas() []Persona {
	if a.store == nil {
		return []Persona{}
	}
	list, err := a.store.GetPersonas()
	if err != nil {
		return []Persona{}
	}
	if list == nil {
		return []Persona{}
	}
	return list
}

// CreatePersona adds a persona. Name must be non-empty after trim.
func (a *App) CreatePersona(name, prompt string) (*Persona, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errPersonaNameRequired
	}
	p, err := a.store.CreatePersona(name, prompt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// UpdatePersona updates name and prompt for an existing persona.
func (a *App) UpdatePersona(id int64, name, prompt string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errPersonaNameRequired
	}
	return a.store.UpdatePersona(id, name, prompt)
}

// DeletePersona removes a persona and clears active selection if it pointed at this id.
func (a *App) DeletePersona(id int64) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	activeStr, _ := a.store.GetSetting(settingActivePersonaID)
	if activeStr != "" {
		if idStr := strconv.FormatInt(id, 10); activeStr == idStr {
			_ = a.store.DeleteSetting(settingActivePersonaID)
		}
	}
	return a.store.DeletePersona(id)
}

// GetActivePersonaID returns the persisted selection, or "" for none.
func (a *App) GetActivePersonaID() string {
	if a.store == nil {
		return ""
	}
	s, _ := a.store.GetSetting(settingActivePersonaID)
	return s
}

// SetActivePersonaID sets the global active persona; empty string means none (no prefix when 2+ personas).
func (a *App) SetActivePersonaID(id string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return a.store.DeleteSetting(settingActivePersonaID)
	}
	pid, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return errInvalidPersonaID
	}
	list, err := a.store.GetPersonas()
	if err != nil {
		return err
	}
	found := false
	for _, p := range list {
		if p.ID == pid {
			found = true
			break
		}
	}
	if !found {
		return errPersonaNotFound
	}
	return a.store.SetSetting(settingActivePersonaID, id)
}
