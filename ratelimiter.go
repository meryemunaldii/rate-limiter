package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// rdb değişkeni bizim Redis bağlantımızı tutacak.
// Başına * koyduk çünkü bu bir "Pointer" (İşaretçi). Bellekteki tek bir Redis bağlantısını temsil ediyor.
var rdb *redis.Client
var ctx = context.Background()

// rateLimiterHandler gelen her isteği karşılayan ve sınırlandıran akıllı fonksiyondur
func rateLimiterHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// 1. İsteği atan kullanıcının IP adresini yakalıyoruz
	userIP := r.RemoteAddr

	// 2. Redis'teki sayacı 1 artırıyoruz (IP -> Key oluyor)
	count, err := rdb.Incr(ctx, userIP).Result()
	if err != nil {
		http.Error(w, "Veritabanı hatası", http.StatusInternalServerError)
		return
	}

	if count == 1 {
		rdb.Expire(ctx, userIP, 1*time.Minute)
	}

	// Arkada takip edebilmek için VS Code terminaline de kimin kaçıncı isteği attığını yazalım:
	fmt.Printf("[LOG] İstek Atan IP: %s | Toplam İstek Sayısı: %d\n", userIP, count)

	// 3. Sınır Kontrolü (Hakkı bittiyse)
	if count > 5 {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, "429 - Çok Fazla İstek Attınız!\n\n")
		fmt.Fprintf(w, "Sistem Kaydı:\n")
		fmt.Fprintf(w, "-> Sizin IP Adresiniz (Key): %s\n", userIP)
		fmt.Fprintf(w, "-> Bu dakikadaki toplam istek sayınız: %d (Sınır: 5)\n", count)
		return
	}

	// 4. Sınırı aşmadıysa (Başarılıysa)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Hoş geldiniz.\n\n")
	fmt.Fprintf(w, "Sistem Kaydı:\n")
	fmt.Fprintf(w, "-> Sizin IP Adresiniz (Key): %s\n", userIP)
	fmt.Fprintf(w, "-> Bu dakikadaki toplam istek sayınız: %d/5\n", count)
}

func main() {
	// 1. Docker'da dün ayağa kaldırdığımız Redis sunucusuna pointer bağlantısı kuruyoruz
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Dün açtığımız port köprüsü
		Password: "",               // Şifre belirlemediğimiz için boş bırakıyoruz
		DB:       0,                // Varsayılan veritabanı odası
	})

	// 2. Bağlantının gerçekten çalışıp çalışmadığını test ediyoruz (Ping-Pong testi)
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		fmt.Println("Docker Redis sunucusuna bağlanılamadı! Lütfen Docker'ın açık olduğundan emin olun.", err)
		return
	}
	fmt.Println("Docker Redis bağlantısı başarıyla kuruldu! 🚀")

	// "/" endpoint'ine gelen istekleri bizim akıllı rateLimiterHandler fonksiyonuna yönlendiriyoruz
	http.HandleFunc("/", rateLimiterHandler)

	fmt.Println("Sunucu 8080 portunda başlatılıyor... http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Sunucu başlatılırken hata oldu:", err)
	}
}
