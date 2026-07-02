package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client //rdb:redis database
var ctx = context.Background()

func rateLimiterHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	//İsteği atan kullanıcının IP adresini yakalıyoruz
	userIP := r.RemoteAddr

	//Redisteki sayacı 1 artırıyoruz (IP -> Key oluyor)
	count, err := rdb.Incr(ctx, userIP).Result() //redisin komutu Incr(increment)
	if err != nil {
		http.Error(w, "Veritabanı hatası", http.StatusInternalServerError)
		return //key e bak eğer bu ip daha önce geldiyse +1 yap. gelmediyse otomatik 1 yap
	}

	if count == 1 {
		rdb.Expire(ctx, userIP, 1*time.Minute) //expire,hafızadaki ömrünü ayarlıyor
	} //1 dk dolduğunda otomatik sıfırlar

	fmt.Printf("[LOG] İstek Atan IP: %s | Toplam İstek Sayısı: %d\n", userIP, count)

	// kontrol kısmı
	if count > 5 {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, "429 - Çok Fazla İstek Attınız!\n\n")
		fmt.Fprintf(w, "Sistem Kaydı:\n")
		fmt.Fprintf(w, "-> Sizin IP Adresiniz (Key): %s\n", userIP)
		fmt.Fprintf(w, "-> Bir dakikadaki toplam istek sayınız: %d (Sınır: 5)\n", count)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "200 - Başarılı! Hoş geldiniz.\n\n")
	fmt.Fprintf(w, "Sistem Kaydı:\n")
	fmt.Fprintf(w, "-> Sizin IP Adresiniz (Key): %s\n", userIP)
	fmt.Fprintf(w, "-> Bir dakikadaki toplam istek sayınız: %d/5\n", count)
}

func main() {

	rdb = redis.NewClient(&redis.Options{
		Addr:     "rate-limiter-redis:6379", //redise bağlandığımız port
		Password: "",
		DB:       0,
	})

	//6379 kapısından docker daki redise bir pinpon fırlatıyor gibi. ping gönderiyor
	//eğer docker açık ve redis çalışıyorsa pong cevabı döner.
	_, err := rdb.Ping(ctx).Result() //baştaki "_" pong cevbını çöpe atıyor. kaydetmiyor gereksşz diye
	if err != nil {
		fmt.Println("Docker Redis sunucusuna bağlanılamadı! Lütfen Docker'ın açık olduğundan emin olun.", err)
		return
	}
	fmt.Println("Docker Redis bağlantısı başarıyla kuruldu!")

	// "/" endpoint'ine gelen istekleri bizim akıllı rateLimiterHandler fonksiyonuna yönlendiriyoruz
	http.HandleFunc("/", rateLimiterHandler)

	fmt.Println("Sunucu 8080 portunda başlatılıyor... http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Sunucu başlatılırken hata oldu:", err)
	}
}
