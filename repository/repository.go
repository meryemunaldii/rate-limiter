package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/redis/go-redis/v9"

	"rate-limiter-project/domain"
)

// Repository tüm veritabanı ve redis işlemlerini tutan yapıdır
type Repository struct {
	db  *sql.DB
	rdb *redis.Client
	ctx context.Context
}

// NewRepository yeni bir repository örneği oluşturur
func NewRepository(db *sql.DB, rdb *redis.Client) *Repository {
	return &Repository{
		db:  db,
		rdb: rdb,
		ctx: context.Background(),
	}
}

// LogToPostgreSQL istek loglarını PostgreSQL'e kaydeder
func (r *Repository) LogToPostgreSQL(ip, endpoint, userAgent string, statusCode int) error {
	query := `INSERT INTO request_logs (ip_address, endpoint, user_agent, status_code, created_at)
              VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.Exec(query, ip, endpoint, userAgent, statusCode, time.Now())
	return err
}

// GetTopEndpoints endpointleri listeler.
func (r *Repository) GetTopEndpoints() ([]domain.TopEndpoint, error) {
	query := `SELECT endpoint, COUNT(*) as count 
	FROM request_logs
	GROUP BY endpoint 
	ORDER BY count DESC;`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []domain.TopEndpoint
	for rows.Next() {
		var e domain.TopEndpoint
		if err := rows.Scan(&e.Endpoint, &e.Count); err == nil {
			endpoints = append(endpoints, e)
		}
	}
	return endpoints, nil
}

// GetTopIPEndpointPairs en çok istek atan IP-Endpoint çiftlerini getirir
func (r *Repository) GetTopIPEndpointPairs() ([]domain.TopPair, error) {
	query := `SELECT ip_address, endpoint, COUNT(*) as total_requests 
              FROM request_logs 
              GROUP BY ip_address, endpoint 
              ORDER BY total_requests DESC LIMIT 5;`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []domain.TopPair
	for rows.Next() {
		var p domain.TopPair //verileri karşılamak için TopPair yapısından geçici bir nesne üretiyoruz.
		if err := rows.Scan(&p.IPAddress, &p.Endpoint, &p.TotalRequests); err == nil {
			pairs = append(pairs, p)
		}
	}
	return pairs, nil
}

// GetIPHistory belirli bir IP'nin geçmiş analizini getirir
func (r *Repository) GetIPHistory(targetIP string) ([]domain.IPStat, error) {
	query := `SELECT endpoint, status_code, COUNT(*) as request_count 
              FROM request_logs WHERE ip_address = $1
              GROUP BY endpoint, status_code ORDER BY request_count DESC;`

	rows, err := r.db.Query(query, targetIP)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []domain.IPStat
	for rows.Next() {
		var s domain.IPStat
		if err := rows.Scan(&s.Endpoint, &s.StatusCode, &s.RequestCount); err == nil {
			stats = append(stats, s)
		}
	}
	return stats, nil
}

// REDİS FONKSİYONLARI
// GetRedisKey redisten bir anahtarın değerini okur
func (r *Repository) GetRedisKey(key string) (string, error) {
	return r.rdb.Get(r.ctx, key).Result()
}

// SetRedisKey redise veri yazar (istek sınırları için)
func (r *Repository) SetRedisKey(key string, value interface{}, expiration time.Duration) error {
	return r.rdb.Set(r.ctx, key, value, expiration).Err()
}

// IncrementRedisKey verilen anahtarın değerini 1 artırır
func (r *Repository) IncrementRedisKey(key string) (int64, error) {
	return r.rdb.Incr(r.ctx, key).Result()
}

// SetExpireRedisKey anahtara ömür biçer
func (r *Repository) SetExpireRedisKey(key string, expiration time.Duration) error {
	return r.rdb.Expire(r.ctx, key, expiration).Err()
}
