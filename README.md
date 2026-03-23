# `prt` - Açık Portları Listele ve Kapat

`prt`, sistemde dinlenen (LISTEN) açık TCP portlarını hızlıca listelemek ve istenen porttaki süreci kapatmak için basit bir CLI aracıdır. Özellikle “Redis/Postgres gibi servisleri açıkta unutuyorum” senaryolarında iş görür.

## Destek

- macOS ve Linux: desteklenir (listeleme için `lsof` kullanır)
- Windows: bu sürüm desteklemez (bu aracın yaptığı port taraması/kill işlemleri burada çalışmaz)

> Not: Süreçleri kapatmak risklidir. Kullanmadan önce port numarasını ve süreci doğru doğruladığından emin ol.

## Kurulum

### 1) Kaynaktan derle (önerilen)

```bash
cd /path/to/network-app
go install .
```

Binary varsayılan olarak `prt` adıyla kurulur.

Eğer komut bulunamadı hatası alırsan `$HOME/go/bin` PATH içinde olmayabilir:

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

### Gereksinimler

- macOS: `lsof` genelde hazır gelir.
- Linux: `lsof` kurulu olmalı (`sudo apt-get install lsof` gibi).
- macOS’ta Homebrew ile yönetilen servisleri kapatmak için: servis Homebrew `brew services` ile çalışıyorsa, `prt kill` kapatmayı `brew services stop <servis>` üzerinden yapar.

## Kullanım

### Açık portları listele

```bash
prt list
```

Çıktıda `PORT`, `PID`, `PROCESS`, `USER` ve (Homebrew ile yönetiliyorsa) `SERVICE` görürsün.

### Portu kapat

```bash
prt kill <port>
```

Varsayılan olarak kapatmadan önce onay sorar.

Zorla kapat (SIGKILL):

```bash
prt kill <port> -f
```

## `prt` yerine başka isim kullanmak istiyorum

İki pratik yol var:

### 1) Sadece binary adını değiştirmek (en basit)

```bash
go build -o myname .
./myname list
./myname kill 6379
```

### 2) CLI ismini ve global install adını değiştirmek (kalıcı)

`go install .` ile global kurulum binary adını modül yolunun son parçasından alır.

Ad değiştirmek için:

1. `go.mod` dosyasındaki `module ...` satırını kendi isim/namespace’inle değiştir.
   - Örn: `module github.com/kullanici/myname`
2. `cmd/root.go` içinde `Use: "prt"` değerini istediğin isimle güncelle.
3. Sonra tekrar:
   ```bash
   go install .
   ```

Bu şekilde hem `--help` çıktısında görünen komut adı hem de global binary adı senin belirlediğin isim olur.

