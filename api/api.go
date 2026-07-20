// http handler lar burada olacak
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"rate-limiter-project/service"
	"strings"

	"github.com/golang-jwt/jwt/v5" //jwt paketi
)

// Handler HTTP isteklerini karşılayan yapıdır
type Handler struct {
	srv *service.Service
}

// NewHandler yeni bir API handler örneği oluşturur
func NewHandler(srv *service.Service) *Handler {
	return &Handler{
		srv: srv,
	}
}

// LoginRequest gelen JSON verisini karşılamak için struct
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginHandler kullanıcı girişini sağlar ve token döner
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Sadece POST istekleri kabul edilir", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Geçersiz istek gövdesi", http.StatusBadRequest)
		return
	}

	// Servis katmanına kullanıcıyı doğrulatıyoruz
	token, err := h.srv.LoginUser(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Giriş başarılıysa token'ı JSON olarak dönüyoruz
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Giris Basarili!",
		"token":   token,
	})
}

// JWTMiddleware rapor sayfalarını koruyan akıllı güvenlik duvarıdır
func (h *Handler) JWTMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// HTTP Header'dan Authorization bilgisini okuyoruz
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Yetkisiz Erişim: Lütfen giriş yapın (Token bulunamadı)", http.StatusUnauthorized)
			return
		}

		// Header formatı genelde "Bearer <token>" şeklindedir
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Token'ı çözüp doğruluğunu kontrol ediyoruz
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// İmza algoritmasını kontrol ediyoruz
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("beklenmeyen imza metodu: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		// Token geçersizse veya süresi dolmuşsa içeri almıyoruz
		if err != nil || !token.Valid {
			http.Error(w, "Yetkisiz Erişim: Geçersiz veya süresi dolmuş token!", http.StatusUnauthorized)
			return
		}

		// Her şey yolundaysa bir sonraki asıl handler fonksiyonuna geçebilir
		next(w, r)
	}
}

// isteklerin rate limitini kontrol eden yardımcı fonksiyondur
func (h *Handler) RateLimiterMiddleware(w http.ResponseWriter, r *http.Request) bool {
	userIP := r.RemoteAddr
	userAgent := r.Header.Get("User-Agent")
	endpoint := r.URL.Path

	// Kararı ve veritabanı işini servis katmanına paslıyoruz
	allowed, count, err := h.srv.CheckRateLimit(userIP, endpoint, userAgent)
	if err != nil {
		http.Error(w, "Sistem hatası", http.StatusInternalServerError)
		return false
	}

	if !allowed {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, "429 - Çok Fazla İstek Attınız!\n\n")
		fmt.Fprintf(w, "Sistem Kaydı:\n")
		fmt.Fprintf(w, "-> Sizin IP Adresiniz (Key): %s\n", userIP)
		fmt.Fprintf(w, "-> Bir dakikadaki toplam istek sayınız: %d (Sınır: 5)\n", count)
		return false
	}

	return true
}

func (h *Handler) HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !h.RateLimiterMiddleware(w, r) {
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Ana Sayfaya Hoş Geldiniz.\n\n")
	fmt.Fprintf(w, "Sistem Kaydı:\n-> IP: %s\n", r.RemoteAddr)
}

func (h *Handler) UsersHandler(w http.ResponseWriter, r *http.Request) {
	if !h.RateLimiterMiddleware(w, r) {
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Kullanıcı listesi yüklendi.\n\n")
}

func (h *Handler) ProductsHandler(w http.ResponseWriter, r *http.Request) {
	if !h.RateLimiterMiddleware(w, r) {
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Ürün listesi yüklendi.\n\n")
}

func (h *Handler) OrdersHandler(w http.ResponseWriter, r *http.Request) {
	if !h.RateLimiterMiddleware(w, r) {
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Sipariş listesi yüklendi.\n\n")
}

func (h *Handler) ReportHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	report, err := h.srv.GetSystemReport()
	if err != nil {
		http.Error(w, "Rapor hazırlanamadı: "+err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(report)
}

func (h *Handler) HistoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	targetIP := r.URL.Query().Get("ip")

	history, err := h.srv.GetIPHistory(targetIP)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(history)
}
