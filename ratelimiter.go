package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"database/sql" //standart sql paketi

	_ "github.com/lib/pq" //postgresql
	"github.com/redis/go-redis/v9"
)

var db *sql.DB        //postgresql db nesnesi
var rdb *redis.Client //rdb:redis database
var ctx = context.Background()

// PostgreSQL'e log yazan fonksiyon
func logToPostgreSQL(ip, endpoint, userAgent string, statusCode int) {
	query := `INSERT INTO request_logs (ip_address, endpoint, user_agent, status_code, created_at) 
	          VALUES ($1, $2, $3, $4, $5)`

	_, err := db.Exec(query, ip, endpoint, userAgent, statusCode, time.Now())
	if err != nil {
		log.Println("PostgreSQL'e log kaydedilirken hata oluştu:", err)
	}
}

func rateLimiterHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	//İsteği atan kullanıcının IP adresini yakalıyoruz
	userIP := r.RemoteAddr
	userAgent := r.Header.Get("User-Agent")

	//Redisteki sayacı 1 artırıyoruz (IP -> Key oluyor)
	count, err := rdb.Incr(ctx, userIP).Result() //redisin komutu Incr(increment)
	if err != nil {
		http.Error(w, "Veritabanı hatası", http.StatusInternalServerError)
		return //key e bak eğer bu ip daha önce geldiyse +1 yap. gelmediyse otomatik 1 yap
	}

	if count == 1 {
		rdb.Expire(ctx, userIP, 1*time.Minute) //expire,hafızadaki ömrünü ayarlıyor
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
		logToPostgreSQL(userIP, r.URL.Path, userAgent, http.StatusTooManyRequests)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Hoş geldiniz.\n\n")
	fmt.Fprintf(w, "Sistem Kaydı:\n")
	fmt.Fprintf(w, "-> Sizin IP Adresiniz (Key): %s\n", userIP)
	fmt.Fprintf(w, "-> Bir dakikadaki toplam istek sayınız: %d/5\n", count)
	//Başarılı isteği 200 koduyla Postgres'e yazıyoruz
	logToPostgreSQL(userIP, r.URL.Path, userAgent, http.StatusOK)
}

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
	dbHost := os.Getenv("DB_HOST")     // docker-compose'da tanımladığımız servis adı (db-service)
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
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Println("Docker PostgreSQL sunucusuna bağlanılamadı!", err)
		return
	}
	log.Println("Docker PostgreSQL bağlantısı başarıyla kuruldu!")

	// "/" endpoint'ine gelen istekleri bizim akıllı rateLimiterHandler fonksiyonuna yönlendiriyoruz
	http.HandleFunc("/", rateLimiterHandler)

	log.Println("Sunucu 8080 portunda başlatılıyor... http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Println("Sunucu başlatılırken hata oldu:", err)
	}
}
