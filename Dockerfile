#İçinde Go 1.26 olan resmi imajı taban alıyoruz
FROM golang:1.26-alpine

#Konteynerın içinde projemizin yaşayacağı bir klasör oluşturuyoruz
WORKDIR /app

#Modül dosyalarımızı konteynerın içine kopyalıyoruz
COPY go.mod go.sum ./

#Kütüphaneleri konteynerın içine indiriyoruz
RUN go mod download

#Kalan bütün kod dosyalarımızı (/app klasörünün içine) kopyalıyoruz
COPY . .

#Kodu derleyip "rate-limiter" adında tek bir çalıştırılabilir dosya yapıyoruz
RUN go build -o rate-limiter .

#Konteyner dış dünyaya 8080 portunu açacak
EXPOSE 8080

#Konteyner ayağa kalktığı an bu derlenen dosyayı çalıştır diyoruz
CMD ["./rate-limiter"]














