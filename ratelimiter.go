package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"fmt"
	"log"
	"net/http"
	"os"

	"database/sql" //standart sql paketi

	_ "github.com/lib/pq" //postgresql
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger"

	"rate-limiter-project/api"
	_ "rate-limiter-project/docs"
	"rate-limiter-project/repository"
	"rate-limiter-project/service"
)

var db *sql.DB        //postgresql db nesnesi
var rdb *redis.Client //rdb:redis database
var ctx = context.Background()

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
	//önce repository kurulur.db bağlantılarını alır.
	repo := repository.NewRepository(db, rdb)

	//servis katmanı kurulur.repository i içine alır.
	srv := service.NewService(repo)

	//api katmanı kurulur. servis katmanını içine alır
	handler := api.NewHandler(srv)

	//yönlendirme
	//kendi özel yönlendiricimizi oluşturduk.
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.HomeHandler) //rotalar artık varsayılan go ya değil. bu mux a kaydediliyor.
	mux.HandleFunc("/api/users", handler.UsersHandler)
	mux.HandleFunc("/api/products", handler.ProductsHandler)
	mux.HandleFunc("/api/orders", handler.OrdersHandler)

	mux.HandleFunc("/swagger/", httpSwagger.WrapHandler)
	mux.HandleFunc("/api/login", handler.LoginHandler) //kullanıcının token alacağı login yolu
	mux.HandleFunc("/api/report", handler.JWTMiddleware(handler.ReportHandler))
	mux.HandleFunc("/api/history", handler.JWTMiddleware(handler.HistoryHandler))

	//Custom HTTP Server Nesnesini Tanımlıyoruz (Graceful Shutdown İçin)
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux, //sunucuya bir istek geldiğinde bu muxu kullan dedik.
	}

	//İşletim sistemi sinyallerini yakalamak için bir kanal oluşturuyoruz
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	//Sunucuyu arka planda başlatıyoruz
	go func() {
		log.Println("Sunucu 8080 portunda başlatılıyor... http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Sunucu beklenmeyen bir hatayla kapandı: %v", err)
		}
	}()

	// Kapanış sinyali (Ctrl+C veya docker stop) gelene kadar burada bekliyoruz
	<-stopChan //kanalın içine bir sinyal düüşene kadar kodun akışını orada kilitler ve bekletir.
	log.Println("\nKapatma sinyali alındı. Graceful Shutdown başlatılıyor...")

	// Aktif isteklerin tamamlanması için 5 saniyelik zaman aşımı veriyoruz
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// HTTP Sunucusunu yeni istek almayacak şekilde durduruyoruz
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP sunucusu kapatılırken hata oluştu: %v", err)
	} else {
		log.Println("HTTP sunucusu yeni istek alımını durdurdu ve mevcut istekleri tamamladı.")
	}

	// Veritabanı ve Redis bağlantılarını graceful kapatıyoruz
	if err := db.Close(); err != nil {
		log.Printf("PostgreSQL bağlantısı kapatılırken hata: %v", err)
	} else {
		log.Println("PostgreSQL veritabanı bağlantısı kapandı.")
	}

	if err := rdb.Close(); err != nil {
		log.Printf("Redis bağlantısı kapatılırken hata: %v", err)
	} else {
		log.Println("Redis bağlantısı kapandı.")
	}

	log.Println("Tüm sistem kaynakları temizlendi. Graceful Shutdown tamamlandı!")
}
