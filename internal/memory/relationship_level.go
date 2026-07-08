package memory

import "time"

func (s *Store) migrateRelLevel() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS rel_level (
			id INT PRIMARY KEY DEFAULT 1,
			minutes_together INT DEFAULT 0,
			updated_at TIMESTAMPTZ DEFAULT now()
		)`)
	if err != nil {
		return err
	}
	// garante a linha única
	_, err = s.db.Exec("INSERT INTO rel_level (id, minutes_together) VALUES (1, 0) ON CONFLICT (id) DO NOTHING")
	return err
}

// AddMinutes soma tempo de convívio e devolve o total acumulado.
func (s *Store) AddMinutes(n int) int {
	s.db.Exec("UPDATE rel_level SET minutes_together = minutes_together + $1, updated_at = $2 WHERE id = 1", n, time.Now())
	var total int
	s.db.QueryRow("SELECT minutes_together FROM rel_level WHERE id = 1").Scan(&total)
	return total
}

// RelMinutes devolve o total de minutos de convívio.
func (s *Store) RelMinutes() int {
	var total int
	s.db.QueryRow("SELECT minutes_together FROM rel_level WHERE id = 1").Scan(&total)
	return total
}


func (s *Store) migrateSkin() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS active_skin (id INT PRIMARY KEY DEFAULT 1, name TEXT DEFAULT 'avatar')`)
	if err != nil {
		return err
	}
	s.db.Exec("INSERT INTO active_skin (id, name) VALUES (1, 'avatar') ON CONFLICT (id) DO NOTHING")
	return nil
}

func (s *Store) SaveSkin(name string) {
	s.db.Exec("UPDATE active_skin SET name=$1 WHERE id=1", name)
}

func (s *Store) LoadSkin() string {
	var n string
	s.db.QueryRow("SELECT name FROM active_skin WHERE id=1").Scan(&n)
	if n == "" {
		return "avatar"
	}
	return n
}