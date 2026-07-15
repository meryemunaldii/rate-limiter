package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"database/sql" //standart sql paketi

	_ "github.com/lib/pq" //postgresql
	"github.com/redis/go-redis/v9"

	_ "rate-limiter-project/docs"

	"rate-limiter-project/repository"

	httpSwagger "github.com/swaggo/http-swagger"
)

var db *sql.DB        //postgresql db nesnesi
var rdb *redis.Client //rdb:redis database
var ctx = context.Background()
var repo *repository.Repository

// PostgreSQL'e log yazan fonksiyon artık Repository'yi kullanıyor
func logToPostgreSQL(ip, endpoint, userAgent string, statusCode int) {
	// Direkt yeni yazdığımız repository fonksiyonunu çağırıyoruz
	err := repo.LogToPostgreSQL(ip, endpoint, userAgent, statusCode)
	if err != nil {
		log.Println("PostgreSQL'e log kaydedilirken hata oluştu:", err)
	}
}

func rateLimiterHandler(w http.ResponseWriter, r *http.Request) bool {
	//İsteği atan kullanıcının IP adresini yakalıyoruz
	userIP := r.RemoteAddr
	userAgent := r.Header.Get("User-Agent")
	endpoint := r.URL.Path

	//Redisteki sayacı 1 artırıyoruz (IP -> Key oluyor)
	count, err := repo.IncrementRedisKey(userIP) //rdb.Incr yerine repository fonxu çağırıyoruz.
	if err != nil {
		http.Error(w, "Veritabanı hatası", http.StatusInternalServerError)
		return false //key e bak eğer bu ip daha önce geldiyse +1 yap. gelmediyse otomatik 1 yap
	}

	if count == 1 {
		repo.SetExpireRedisKey(userIP, 1*time.Minute) //rdb.Expire yerine repository fonxunu çağırıyoruz.
	} //1 dk dolduğunda otomatik sıfırlar

	log.Printf("İstek Atan IP: %s | Toplam İstek Sayısı: %d\n", userIP, count)

	// kontrol kısmı
	if count > 5 {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, "429 - Çok Fazla İstek Attınız!\n\n")
		fmt.Fprintf(w, "Sistem Kaydı:\n")
		fmt.Fprintf(w, "-> Sizin IP Adresiniz (Key): %s\n", userIP)
		fmt.Fprintf(w, "-> Bir dakikadaki toplam istek sayınız: %d (Sınır: 5)\n", count)
		//Engellenen isteği de 429 koduyla Postgres'e yazıyoruz
		logToPostgreSQL(userIP, endpoint, userAgent, http.StatusTooManyRequests)
		return false
	}
	// Limit aşılmadıysa veritabanına 200 yaz ve true dön
	logToPostgreSQL(userIP, endpoint, userAgent, http.StatusOK)
	return true
}
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !rateLimiterHandler(w, r) {
		return
	} // Önce hızı kontrol et, engellendiyse dur

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Ana Sayfaya Hoş Geldiniz.\n\n")
	fmt.Fprintf(w, "Sistem Kaydı:\n-> IP: %s\n", r.RemoteAddr)
}

func UsersHandler(w http.ResponseWriter, r *http.Request) {
	if !rateLimiterHandler(w, r) {
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Kullanıcı listesi yüklendi.\n\n")
	fmt.Fprintf(w, "Sistem Kaydı:\n-> IP: %s\n", r.RemoteAddr)
}

func ProductsHandler(w http.ResponseWriter, r *http.Request) {
	if !rateLimiterHandler(w, r) {
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Ürün listesi yüklendi.\n\n")
	fmt.Fprintf(w, "Sistem Kaydı:\n-> IP: %s\n", r.RemoteAddr)
}

func OrdersHandler(w http.ResponseWriter, r *http.Request) {
	if !rateLimiterHandler(w, r) {
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Sipariş listesi yüklendi.\n\n")
	fmt.Fprintf(w, "Sistem Kaydı:\n-> IP: %s\n", r.RemoteAddr)
}

// @title           Rate Limiter API
// @version         1.0
// @description     Go, Redis ve PostgreSQL ile geliştirilmiş gelişmiş Rate Limiter projesi.
// @host            localhost:8080
// @BasePath        /
func main() {

	rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"), //redise bağlandığımız port
		Password: "",
		DB:       0,
	})

	//6379 kapısından docker daki redise bir pinpon fırlatıyor gibi. ping gönderiyor
	//eğer docker açık ve redis çalışıyorsa pong cevabı döner.
	_, err := rdb.Ping(ctx).Result() //baştaki "_" pong cevbını çöpe atıyor. kaydetmiyor gereksşz diye
	if err != nil {
		log.Println("Docker Redis sunucusuna bağlanılamadı! Lütfen Docker'ın açık olduğundan emin olun.", err)
		return
	}
	log.Println("Docker Redis bağlantısı başarıyla kuruldu!")

	//postgresql bağlantı kısmı
	// Bilgileri kodun içine gömmek yerine env den değişkenlerinden çekiyoruz
	dbHost := os.Getenv("DB_HOST")     // docker-compose'da tanımladığımız servis adı
	dbPort := os.Getenv("DB_PORT")     // 5432
	dbUser := os.Getenv("DB_USER")     // meryem_user
	dbPass := os.Getenv("DB_PASSWORD") //
	dbName := os.Getenv("DB_NAME")     // rate_limiter_db

	// Sürücüye göndereceğimiz bağlantı metnini güvenli değişkenlerle oluşturuyoruz
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Println("PostgreSQL sürücü hatası:", err)
		return
	}

	err = db.Ping()
	if err != nil {
		log.Println("Docker PostgreSQL sunucusuna bağlanılamadı!", err)
		return
	}
	log.Println("Docker PostgreSQL bağlantısı başarıyla kuruldu!")

	// db ve rdb bağlantı kodlarından hemen sonra:
	repo = repository.NewRepository(db, rdb)

	//yönlendirme
	http.HandleFunc("/", HomeHandler)
	http.HandleFunc("/api/users", UsersHandler)
	http.HandleFunc("/api/products", ProductsHandler)
	http.HandleFunc("/api/orders", OrdersHandler)
	http.HandleFunc("/api/report", ReportHandler)
	http.HandleFunc("/api/history", HistoryHandler)
	http.HandleFunc("/swagger/", httpSwagger.WrapHandler)

	log.Println("Sunucu 8080 portunda başlatılıyor... http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Println("Sunucu başlatılırken hata oldu:", err)
	}
}

func ReportHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json") // json verisi gidiyor

	// 1. En çok istek atan IP-Endpoint çiftlerini repodan çekiyoruz
	topPairs, err := repo.GetTopIPEndpointPairs()
	if err != nil {
		http.Error(w, "Rapor hatası: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. En popüler endpoint'leri repodan çekiyoruz
	topEndpoints, err := repo.GetTopEndpoints()
	if err != nil {
		http.Error(w, "Rapor hatası: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// JSON Çıktısı oluşturma
	reportResult := map[string]interface{}{
		"title":                 "Sistem İstek Raporu",
		"top_ip_endpoint_pairs": topPairs,
		"popular_endpoints":     topEndpoints,
	}
	json.NewEncoder(w).Encode(reportResult)
}

// HistoryHandler Go projesinin geçmiş istek analitiğini döner
// @Summary      IP Geçmiş Analitiği Sorgulama
// @Description  Belirtilen IP adresinin hangi endpoint'e kaç istek attığını ve aldığı durum kodlarını listeler.
// @Tags         Analitik
// @Accept       json
// @Produce      json
// @Param        ip   query     string  true  "Sorgulanacak IP Adresi (Örn: 127.0.0.1)"
// @Success      200  {string}  string  "Başarılı Rapor Çıktısı"
// @Failure      400  {string}  string  "Eksik parametre hatası"
// @Failure      500  {string}  string  "Veritabanı sorgu hatası"
// @Router       /api/history [get]
func HistoryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json") // Gönderdiğim veri json paketi

	// URL'den ip parametresini çekiyoruz
	targetIP := r.URL.Query().Get("ip")

	if targetIP == "" {
		http.Error(w, `{"error": "Lütfen sorgulamak için bir 'ip' parametresi girin. Örn: /api/history?ip=127.0.0.1"}`, http.StatusBadRequest)
		return
	}

	// 3. O ip ye ait geçmiş analitiğini repodan tek satırda çekiyoruz
	stats, err := repo.GetIPHistory(targetIP)
	if err != nil {
		http.Error(w, `{"error": "Veritabanı sorgu hatası"}`, http.StatusInternalServerError)
		return
	}

	// Sonucu json formatında hazırlayıp dışarıya fırlatıyoruz
	response := map[string]interface{}{
		"searched_ip":   targetIP,
		"total_records": len(stats),
		"statistics":    stats,
	}

	json.NewEncoder(w).Encode(response)
}
