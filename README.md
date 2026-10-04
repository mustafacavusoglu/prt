# `prt` — Açık Portları Listele ve Kapat

[English](README.en.md)

`prt`, sistemde dinlenen TCP/UDP portlarını ve onları açan süreçleri hızlıca gösteren, istediğini güvenle kapatan küçük bir CLI aracıdır. “Redis/Postgres’i açıkta unuttum” senaryosu için tasarlandı.

- **Hızlı:** Linux’ta `lsof`’a ihtiyaç duymadan doğrudan `/proc` okur; macOS’ta tüm süreç bilgilerini toplu (tek `lsof`/`ps`/`launchctl` çağrısıyla) toplar.
- **Güvenli:** onay ister, ne kapatılacağını gösterir, PID 1/kendi sürecini/`sshd`/`systemd`/`launchd` gibi sistem süreçlerini reddeder, sinyalden sonra sürecin gerçekten kapandığını doğrular.
- **Kullanımı kolay:** interaktif seçim modu, port aralıkları, filtreler, `--json`, kabuk tamamlama.
- **Dikkat çekici:** ağdan erişilebilen (`0.0.0.0`/LAN) portlar sarı renkle işaretlenir; `--exposed` ile yalnızca onlar listelenir.

## Destek

| Platform | Port tarama | Notlar |
|---|---|---|
| Linux | `/proc` (bağımlılık yok) | Başka kullanıcıların süreçleri için `sudo` gerekir |
| macOS | `lsof` + `ps` | Homebrew servisleri `brew services stop` ile durdurulur |
| Windows | `netstat` + `tasklist` | Kullanıcı/komut satırı/dizin bilgisi gösterilmez; kapatma `taskkill` ile yapılır |

## Kurulum

```bash
# Kaynaktan (Go 1.24+)
go install github.com/mustafacavusoglu/prt@latest

# veya depoyu klonlayıp
make install        # sürüm bilgisiyle kurar
make build          # ./prt üretir
```

`prt` komutu bulunamazsa `$(go env GOPATH)/bin` dizinini PATH’e ekleyin:

```bash
echo 'export PATH="$(go env GOPATH)/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

Hazır binary’ler için [Releases](https://github.com/mustafacavusoglu/prt/releases) sayfasına bakın (GoReleaser ile `v*` etiketinde üretilir).

Kabuk tamamlama (port numaraları dinamik tamamlanır):

```bash
prt completion zsh  > "${fpath[1]}/_prt"        # zsh
prt completion bash > /etc/bash_completion.d/prt # bash
prt completion fish > ~/.config/fish/completions/prt.fish
```

## Kullanım

### Listele

```bash
prt list                       # kendi süreçlerinizin dinlediği portlar
prt list 6379                  # belirli port(lar) / aralık: prt list 3000-3010,5432
prt list --name redis          # süreç adı veya komut satırına göre ara
prt list --exposed             # yalnızca ağdan erişilebilenler (0.0.0.0, LAN IP)
prt list --all                 # diğer kullanıcıların süreçleri de (root’ta varsayılan)
prt list --udp                 # UDP soketleri de
prt list --wide                # çalışma süresi, çalışma dizini, tam komut satırı
prt list --json | jq '.[] | select(.exposed) | .port'
```

Örnek çıktı:

```
PORT    PROTO   PID    PROCESS        USER   ADDRESS     SERVICE
5432    tcp     812    postgres       mus    127.0.0.1   brew:postgresql@16
6379    tcp     1204   redis-server   mus    *           brew:redis      ← sarı: ağa açık
8080    tcp     5531   node           mus    127.0.0.1   -

Toplam: 3 port (1 tanesi ağa açık)
```

`ADDRESS` sütununda `*` “tüm arayüzler” demektir. Loopback (`127.0.0.1`, `::1`) dışındaki her adres “ağa açık” sayılır.

### Kapat

```bash
prt kill 6379                  # SIGTERM + onay; kapandığını doğrular
prt kill 3000 5432 8080        # birden fazla port
prt kill 3000-3010 --yes       # aralık, onaysız
prt kill 8080 --dry-run        # ne olacağını göster, hiçbir şey yapma
prt kill 8080 -f               # SIGKILL
prt kill 8080 -s HUP           # başka sinyal: TERM, KILL, HUP, INT
prt kill 6379 --no-brew        # brew servisini atlayıp doğrudan sinyal gönder
```

- Bir port birden çok süreç tarafından tutuluyorsa (ör. nginx worker’ları) **hepsi** gösterilir ve birlikte kapatılır; aynı süreç birden çok portu tutuyorsa tek kez kapatılır.
- Süreç `--timeout` (varsayılan 3 sn) içinde kapanmazsa uyarılır: `prt kill 8080 -f`.
- Onay sorulduktan sonra süreç hâlâ o portu dinliyor mu yeniden kontrol edilir (PID yeniden kullanımına karşı).
- Linux’ta başka kullanıcıya ait portlar `PID -` olarak görünür; kapatmak için `sudo prt kill <port>`.
- Korumalı süreçler (PID 1, `prt`’nin kendisi: asla; `sshd`, `systemd`, `launchd`, `dockerd`…: `--unsafe` ile) reddedilir.

### İnteraktif mod

```bash
prt                            # terminalde argümansız çalıştırınca
prt interactive                # (kısa: prt i)
```

Portlar numaralı listelenir; `1,3` veya `2-4` yazarak kapatılacakları seçersiniz, ardından onay istenir.

### Çıkış kodları

| Kod | Anlamı |
|---|---|
| 0 | Başarılı |
| 1 | Genel hata (geçersiz argüman vb.) |
| 2 | Port üzerinde dinleyen süreç bulunamadı |
| 3 | Yetki yok (sudo gerekir) |
| 4 | Kullanıcı onay vermedi / onay alınamadı |
| 5 | Sinyal gönderildi ama süreç kapanmadı |

Etkileşimsiz ortamda (CI, cron) onay istenemeyeceği için `--yes` kullanın. Renkleri `--no-color` veya `NO_COLOR=1` kapatır.

## Geliştirme

```bash
make test      # go test -race ./...
make lint      # vet (linux/darwin/windows) + gofmt kontrolü
make cover     # kapsam özeti
```

Yapı: `cmd/` cobra komutları ve çıktı biçimi, `internal/port/` platforma özel tarama ve süreç sonlandırma (`scan_linux.go`, `scan_lsof.go`, `scan_windows.go`). Çıktı ayrıştırıcıları platformdan bağımsız dosyalardadır, böylece her platformun testi her yerde çalışır.

Sürüm çıkarmak için `git tag v0.1.0 && git push --tags` yeterli; GitHub Actions binary’leri üretip Releases’e yükler. Homebrew için `.goreleaser.yaml` içindeki `brews` bloğunu etkinleştirin (önce `homebrew-tap` reposunu açın).

> Süreçleri kapatmak risklidir. Kapatmadan önce portu ve süreci doğrulayın.

## `prt` yerine başka isim

Binary adını değiştirmek için `go build -o myname .` yeterlidir. Kalıcı olarak değiştirmek için `go.mod`’daki `module` satırını, import yollarını ve `cmd/root.go` içindeki `Use: "prt"` değerini güncelleyin.

## Lisans

[MIT](LICENSE)
