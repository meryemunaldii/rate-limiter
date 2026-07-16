// http handler lar burada olacak
package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"rate-limiter-project/service"
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
