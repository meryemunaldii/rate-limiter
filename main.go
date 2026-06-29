package main

import (
	"fmt"
	"net/http"
)

// helloHandler gelen istekleri karşılayan fonksiyondur
func helloHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return //Eğer kullanıcı tam olarak ana sayfaya (/) gelmediyse, gidip arkaya garip bir şeyler yazdıysa ona 200 dönme, 404 Sayfa Bulunamadı (http.NotFound) hatası dön.
	}

	// HTTP durum kodunu 200 OK olarak ayarlıyoruz
	w.WriteHeader(http.StatusOK)

	// Tarayıcıya yanıt metni gönderiyoruz
	fmt.Fprint(w, "200 - OK. Sunucu başarıyla çalışıyor!")
}

func main() {
	// "/" (endpoint) gelen istekleri helloHandler fonxuna yönlendiriyoruz
	http.HandleFunc("/", helloHandler)

	fmt.Println("Sunucu 8080 portunda başlatılıyor... http://localhost:8080")

	// err handling
	err := http.ListenAndServe(":8080", nil) //bu dinleme kodunu en alta yazıyoruz çünkü dinleme işlemini hep yapıyor.
	if err != nil {
		fmt.Println("Sunucu başlatılırken hata oluştu:", err)
	}
}
