// iş mantığı burada olacak
package service

import (
	"errors"
	"os"
	"time"

	"rate-limiter-project/repository"

	"github.com/golang-jwt/jwt/v5" //jwt paketi
)

// Servisin gizli imza anahtarı (Secret Key).
var jwtSecretKey = []byte("gizli_anahtar")

// Service iş mantığı işlemlerini yürüten yapıdır
type Service struct {
	repo *repository.Repository
}

// NewService yeni bir servis örneği oluşturur
func NewService(repo *repository.Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// LoginUser kullanıcı bilgilerini doğrular ve token üretir
func (s *Service) LoginUser(username, password string) (string, error) {
	expectedUser := os.Getenv("APP_USER")
	expectedPassword := os.Getenv("APP_PASSWORD")
	jwtSecret := os.Getenv("JWT_SECRET")

	if expectedUser == "" || expectedPassword == "" || jwtSecret == "" {
		return "", errors.New("sistemsel yapılandırma hatası: çevre değişkenleri eksik")
	}

	// Şimdilik veritabanı yükü oluşturmamak için sabit kullanıcı adı ve şifre kontrolü
	// İleride veritabanından çekilecek şekilde genişletilebilir
	if username == expectedUser && password == expectedPassword {

		// Token içinde taşıyacağımız verileri (claims) hazırlıyoruz
		claims := jwt.MapClaims{
			"username": username,
			"role":     "admin",
			"exp":      time.Now().Add(time.Hour * 24).Unix(), // 24 Saat sonra süresi dolacak
		}

		// HS256 algoritmasıyla token'ı oluşturup gizli anahtarımızla imzalıyoruz
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(jwtSecretKey)
		if err != nil {
			return "", err
		}
		return tokenString, nil
	}

	return "", errors.New("hatali kullanici adi veya sifre")
}

// CheckRateLimit gelen IP adresi için limit kontrolü ve loglama yapar
// Eğer limit aşılmadıysa true, aşıldıysa false döner
func (s *Service) CheckRateLimit(ip, endpoint, userAgent string) (bool, int, error) {
	count, err := s.repo.IncrementRedisKey(ip)
	if err != nil {
		return false, 0, err
	}

	if count == 1 {
		s.repo.SetExpireRedisKey(ip, 1*time.Minute)
	}

	// Limit aşımı kontrolü
	if count > 5 {
		// Engellenen isteği 429 koduyla PostgreSQL e logla
		s.repo.LogToPostgreSQL(ip, endpoint, userAgent, 429)
		return false, int(count), nil
	}

	// Limit aşılmadıysa 200 koduyla PostgreSQL e logla
	s.repo.LogToPostgreSQL(ip, endpoint, userAgent, 200)
	return true, int(count), nil
}

// raporlama verilerini hazırlar
func (s *Service) GetSystemReport() (map[string]interface{}, error) {
	topPairs, err := s.repo.GetTopIPEndpointPairs()
	if err != nil {
		return nil, err
	}

	topEndpoints, err := s.repo.GetTopEndpoints()
	if err != nil {
		return nil, err
	}

	reportResult := map[string]interface{}{
		"title":                 "Sistem İstek Raporu",
		"top_ip_endpoint_pairs": topPairs,
		"popular_endpoints":     topEndpoints,
	}

	return reportResult, nil
}

// IP analitiği verilerini çeker
func (s *Service) GetIPHistory(ip string) (map[string]interface{}, error) {
	if ip == "" {
		return nil, errors.New("IP parametresi boş olamaz")
	}

	stats, err := s.repo.GetIPHistory(ip)
	if err != nil {
		return nil, err
	}

	response := map[string]interface{}{
		"searched_ip":   ip,
		"total_records": len(stats),
		"statistics":    stats,
	}

	return response, nil
}
