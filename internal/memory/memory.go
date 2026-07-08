package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	rdb *redis.Client
	db  *sql.DB
}

func New(redisAddr, pgDSN string) (*Store, error) {
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}
	db, err := sql.Open("postgres", pgDSN)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	s := &Store{rdb: rdb, db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	if err := s.migratePersonas(); err != nil {
		return nil, err
	}
	if err := s.migrateRelationship(); err != nil {
		return nil, err
	}
	if err := s.migrateViewers(); err != nil {
		return nil, err
	}
	if err := s.migrateRelLevel(); err != nil {
		return nil, err
	}
	return s, s.migrateSkin()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS facts (
			id SERIAL PRIMARY KEY,
			content TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT now()
		)`)
	return err
}

const sessionKey = "backseat:session"

func (s *Store) AppendSession(role, text string) {
	ctx := context.Background()
	s.rdb.RPush(ctx, sessionKey, role+": "+text)
	s.rdb.LTrim(ctx, sessionKey, -100, -1)
	s.rdb.Expire(ctx, sessionKey, 24*time.Hour)
}

func (s *Store) SessionTranscript() string {
	items, err := s.rdb.LRange(context.Background(), sessionKey, 0, -1).Result()
	if err != nil || len(items) == 0 {
		return ""
	}
	return strings.Join(items, "\n")
}

func (s *Store) ClearSession() {
	s.rdb.Del(context.Background(), sessionKey)
}

func (s *Store) AddFact(content string) error {
	_, err := s.db.Exec("INSERT INTO facts (content) VALUES ($1)", content)
	return err
}

func (s *Store) Facts(n int) []string {
	rows, err := s.db.Query("SELECT content FROM facts ORDER BY id DESC LIMIT $1", n)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if rows.Scan(&c) == nil {
			out = append(out, c)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
