package memory

// Persona é um personagem salvo que a Dora pode encarnar.
type Persona struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
	Active bool   `json:"active"`
}

func (s *Store) migratePersonas() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS personas (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			prompt TEXT NOT NULL,
			active BOOLEAN DEFAULT false,
			created_at TIMESTAMPTZ DEFAULT now()
		)`)
	return err
}

func (s *Store) ListPersonas() ([]Persona, error) {
	rows, err := s.db.Query("SELECT id, name, prompt, active FROM personas ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Persona
	for rows.Next() {
		var p Persona
		if err := rows.Scan(&p.ID, &p.Name, &p.Prompt, &p.Active); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Store) CreatePersona(name, prompt string) (int, error) {
	var id int
	err := s.db.QueryRow("INSERT INTO personas (name, prompt) VALUES ($1,$2) RETURNING id", name, prompt).Scan(&id)
	return id, err
}

func (s *Store) UpdatePersona(id int, name, prompt string) error {
	_, err := s.db.Exec("UPDATE personas SET name=$1, prompt=$2 WHERE id=$3", name, prompt, id)
	return err
}

func (s *Store) DeletePersona(id int) error {
	_, err := s.db.Exec("DELETE FROM personas WHERE id=$1", id)
	return err
}

// ActivatePersona marca uma como ativa (e desativa as outras). Devolve o prompt dela.
func (s *Store) ActivatePersona(id int) (string, error) {
	if _, err := s.db.Exec("UPDATE personas SET active=false"); err != nil {
		return "", err
	}
	var prompt string
	err := s.db.QueryRow("UPDATE personas SET active=true WHERE id=$1 RETURNING prompt", id).Scan(&prompt)
	return prompt, err
}

// ActivePersona devolve o prompt da persona ativa ("" se nenhuma).
func (s *Store) ActivePersona() string {
	var prompt string
	s.db.QueryRow("SELECT prompt FROM personas WHERE active=true LIMIT 1").Scan(&prompt)
	return prompt
}
