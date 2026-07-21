package main

import (
	"context"

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
	http.HandleFunc("/", handler.HomeHandler)
	http.HandleFunc("/api/users", handler.UsersHandler)
	http.HandleFunc("/api/products", handler.ProductsHandler)
	http.HandleFunc("/api/orders", handler.OrdersHandler)

	http.HandleFunc("/swagger/", httpSwagger.WrapHandler)
	http.HandleFunc("/api/login", handler.LoginHandler) //kullanıcının token alacağı login yolu
	http.HandleFunc("/api/report", handler.JWTMiddleware(handler.ReportHandler))
	http.HandleFunc("/api/history", handler.JWTMiddleware(handler.HistoryHandler))

	log.Println("Sunucu 8080 portunda başlatılıyor... http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Println("Sunucu başlatılırken hata oldu:", err)
	}
}
