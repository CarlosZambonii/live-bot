package memory

func (s *Store) migrateRelationship() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS relationship (
			id SERIAL PRIMARY KEY,
			observation TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT now()
		)`)
	return err
}

func (s *Store) AddObservation(obs string) error {
	_, err := s.db.Exec("INSERT INTO relationship (observation) VALUES ($1)", obs)
	return err
}

func (s *Store) Observations(n int) []string {
	rows, err := s.db.Query("SELECT observation FROM relationship ORDER BY id DESC LIMIT $1", n)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var o string
		if rows.Scan(&o) == nil {
			out = append(out, o)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
