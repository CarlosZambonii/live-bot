package memory

func (s *Store) migrateViewers() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS viewers (
			nick TEXT PRIMARY KEY,
			messages INT DEFAULT 0,
			first_seen TIMESTAMPTZ DEFAULT now(),
			last_seen TIMESTAMPTZ DEFAULT now()
		)`)
	return err
}

// SeeViewer registra que um viewer falou; devolve true se é regular (visto antes).
func (s *Store) SeeViewer(nick string) bool {
	var msgs int
	s.db.QueryRow("SELECT messages FROM viewers WHERE nick=$1", nick).Scan(&msgs)
	s.db.Exec(`
		INSERT INTO viewers (nick, messages) VALUES ($1, 1)
		ON CONFLICT (nick) DO UPDATE SET messages = viewers.messages + 1, last_seen = now()`, nick)
	return msgs >= 5 // 5+ mensagens acumuladas = regular
}

// TopViewers devolve os nicks mais ativos (pro contexto da Dora).
func (s *Store) TopViewers(n int) []string {
	rows, err := s.db.Query("SELECT nick FROM viewers ORDER BY messages DESC LIMIT $1", n)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var nk string
		if rows.Scan(&nk) == nil {
			out = append(out, nk)
		}
	}
	return out
}
