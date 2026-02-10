# Go SSH Host Manager

SSH hostlarını ağaç yapısında yönetmek ve bağlanmak için tam ekran CLI grafik arayüzlü Go uygulaması.

## Özellikler

- 🎨 Tam ekran terminal arayüzü (TUI)
- 🌳 Ağaç yapısında kategori ve host organizasyonu
- 📁 İç içe kategoriler desteği
- ⌨️  Ok tuşları ile navigasyon
- 🔗 Çoklu host üzerinden SSH bağlantısı
- � Sıralı komut çalıştırma (karmaşık bağlantı senaryoları için)- 🤖 Interactive mode - Otomatik şifre girme ve komut gönderme- �📝 YAML tabanlı konfigürasyon
- 🏠 Kullanıcı dizininde otomatik config yönetimi

## Kurulum

```bash
go build -o go-ssh
sudo mv go-ssh /usr/local/bin/
```

veya

```bash
go install
```

## Kullanım

Uygulamayı çalıştırın:

```bash
go-ssh
```

İlk çalıştırmada otomatik olarak `~/.go-ssh/config.yaml` dosyası oluşturulacaktır.

### Klavye Kısayolları

| Tuş | Açıklama |
|-----|----------|
| `↑/↓` veya `j/k` | Yukarı/Aşağı navigasyon |
| `←/→` veya `h/l` | Kategori kapat/aç |
| `Enter` veya `Space` | Kategori aç/kapat veya hosta bağlan |
| `e` | Tüm kategorileri genişlet |
| `c` | Tüm kategorileri daralt |
| `q` veya `Ctrl+C` | Çıkış |

## Konfigürasyon

Config dosyası: `~/.go-ssh/config.yaml`

### Ağaç Yapısı

Kategoriler iç içe geçebilir ve her kategori hem alt kategoriler hem de hostlar içerebilir:

```yaml
categories:
  - name: Production
    description: Production environment servers
    icon: "🔴"
    categories:
      - name: Web Servers
        description: Frontend web servers
        icon: "🌐"
        hosts:
          - name: Web Server 1
            description: Primary web server
            command: ssh -t jumphost@bastion "ssh -t deploy@web1 'cd /var/www && exec bash'"
          - name: Web Server 2
            description: Secondary web server
            command: ssh -t jumphost@bastion "ssh -t deploy@web2 'cd /var/www && exec bash'"
      - name: Database Servers
        description: Database servers
        icon: "🗄️"
        hosts:
          - name: MySQL Master
            description: Primary MySQL server
            command: ssh -t jumphost@bastion "ssh -t dba@mysql-master 'exec bash'"
    hosts:
      - name: Bastion Host
        description: Jump server for production
        command: ssh jumphost@bastion

  - name: Staging
    description: Staging environment
    icon: "🟡"
    hosts:
      - name: Staging Server
        description: Staging environment server
        command: ssh deploy@staging

  - name: Development
    description: Development servers
    icon: "🟢"
    categories:
      - name: Local VMs
        description: Local virtual machines
        icon: "💻"
        hosts:
          - name: Dev VM 1
            description: Development VM
            command: ssh dev@192.168.1.100
    hosts:
      - name: Dev Server
        description: Main development server
        command: ssh dev@devserver
```

### Config Yapısı

**Kategori:**
- `name`: Kategori adı
- `description`: Açıklama (opsiyonel)
- `icon`: Emoji ikon (opsiyonel)
- `categories`: Alt kategoriler (opsiyonel)
- `hosts`: Hostlar (opsiyonel)

**Host:**
- `name`: Host'un görünen adı
- `description`: Host açıklaması (opsiyonel)
- `command`: Çalıştırılacak tek SSH komutu (basit bağlantılar için)
- `commands`: Sırayla çalıştırılacak komutlar listesi (karmaşık bağlantılar için)

> **Not:** Bir host için ya `command` ya da `commands` kullanılmalıdır, ikisi birden kullanılamaz.

### Basit Bağlantı Örneği

Tek bir komutla doğrudan bağlantı:

```yaml
hosts:
  - name: Production Server
    description: Main production server
    command: ssh user@production.example.com
```

### Karmaşık Bağlantı Örneği (Sıralı Komutlar)

Jump host üzerinden veya çoklu adımlı bağlantılar için:

```yaml
hosts:
  - name: Inner Server
    description: Server behind jump host
    commands:
      - ssh jumphost@bastion.example.com   # İlk önce bastion'a bağlan
      - sleep 2                             # Bağlantının kurulmasını bekle
      - ssh user@internal-server            # Oradan iç sunucuya bağlan

  - name: Complex Setup
    description: Multi-step connection
    commands:
      - echo "Connecting to production..."
      - ssh -t jump@gateway "cd /opt/scripts && ./prepare.sh"
      - sleep 1
      - ssh -t jump@gateway "ssh app@prod-server"
```

**Sıralı Komutlar Nasıl Çalışır:**
- İlk SSH komutu bulunur ve ona `-tt` flag'i eklenir (terminal allocation için)
- Diğer tüm komutlar, ilk SSH bağlantısı içinde çalıştırılacak remote komutlar olarak embed edilir
- Son komut SSH ise, `exec` ile çalıştırılarak kullanıcı doğrudan o session'a bağlanır
- Örnek: `["ssh host1", "sleep 2", "ssh host2"]` → `ssh -tt host1 'sleep 2; exec ssh host2'`

**Örnek Dönüşüm:**
```yaml
commands:
  - ssh jumphost@bastion
  - sleep 2
  - ssh user@internal-server
```
Bu otomatik olarak şuna dönüştürülür:
```bash
ssh -tt jumphost@bastion 'sleep 2; exec ssh user@internal-server'
```

### Interactive Mode (Otomatik Şifre/Komut Girişi)

Interactive mode, Go uygulamasının SSH bağlantısını PTY (pseudo-terminal) ile yönetmesini sağlar. Bu sayede:
- Otomatik şifre girebilirsiniz
- Bağlantı kurulduktan sonra otomatik komutlar gönderebilirsiniz
- Son olarak kullanıcıya kontrolü verebilirsiniz

**Özel Komut Prefixleri:**
- `SEND:text` - Terminal'e text gönderir (enter ile)
- `WAIT:N` - N saniye bekler
- `INTERACT` - Kullanıcıya kontrolü verir

**Örnek 1: Şifre ile Bağlantı**
```yaml
hosts:
  - name: Server with Password
    description: Auto-login with password
    commands:
      - ssh user@server.com          # SSH başlat
      - WAIT:2                        # Şifre promptu için bekle
      - SEND:mypassword123            # Şifreyi gönder
      - INTERACT                      # Kullanıcıya kontrolü ver
```

**Örnek 2: Şifre + Otomatik Komutlar**
```yaml
hosts:
  - name: Auto Setup Server
    description: Login and run setup commands
    commands:
      - ssh user@server.com
      - WAIT:2
      - SEND:mypassword                # Şifre gönder
      - WAIT:1                         # Prompt için bekle
      - SEND:cd /opt/app               # Dizine geç
      - SEND:./setup.sh                # Script çalıştır
      - INTERACT                       # Kullanıcı devam etsin
```

**Örnek 3: Jump Host ile Karmaşık Senaryo**
```yaml
hosts:
  - name: Multi-Hop with Passwords
    description: Jump through multiple hosts with passwords
    commands:
      - ssh jumphost@bastion.com
      - WAIT:2
      - SEND:bastion_password
      - WAIT:1
      - SEND:ssh user@internal-server
      - WAIT:2
      - SEND:internal_password
      - INTERACT
```

## 🔐 Password Manager

Go-SSH, şifrelerinizi güvenli bir şekilde saklamak için yerleşik bir password manager içerir. Şifreler AES-256-GCM encryption ile şifrelenir ve disk'te güvenli bir şekilde saklanır.

### Password Manager'ı Kullanma

Password manager'ı başlatmak için:

```bash
./go-ssh --passwords
```

İlk çalıştırmada master password oluşturmanız istenecektir. Bu password, tüm kayıtlı şifrelerinizi koruyacaktır.

### Menü Seçenekleri

1. **Add Password**: Yeni şifre ekle
   - ID: Şifreyi tanımlayan benzersiz bir kod (örn: `prod-db`, `staging-app`)
   - Description: Şifre hakkında açıklama
   - Password: Saklanacak şifre

2. **List Passwords**: Kayıtlı şifreleri listele

3. **Remove Password**: Şifre sil

### Config'de SENDPASS Kullanma

Kayıtlı şifreleri SSH bağlantılarında kullanmak için `SENDPASS:password_id` komutunu kullanın:

```yaml
categories:
  - name: Production
    hosts:
      - name: Database Server
        description: Production database with password
        commands:
          - ssh user@db-server.com
          - SENDPASS:prod-db        # Password manager'dan şifreyi gönder
          - INTERACT
```

### Güvenlik Özellikleri

- ✅ AES-256-GCM encryption
- ✅ PBKDF2 key derivation (100,000 iterations)
- ✅ Master password ile şifreleme
- ✅ Disk'te sadece şifreli data
- ✅ 0600 dosya izinleri (sadece owner okuyabilir)
- ✅ Şifreler memory'de sadece gerektiğinde decrypt edilir

### Örnek Workflow

1. Password manager'ı başlat:
   ```bash
   ./go-ssh --passwords
   ```

2. Yeni şifre ekle:
   - ID: `prod-web`
   - Description: `Production web server password`
   - Password: `<your-secure-password>`

3. Config'de kullan:
   ```yaml
   - name: Web Server
     commands:
       - ssh admin@web-server.com
       - SENDPASS:prod-web
       - INTERACT
   ```

4. Normal şekilde go-ssh'i çalıştır:
   ```bash
   ./go-ssh
   ```

5. Host'u seç, master password gir, otomatik login!

**Güvenlik Notu:** Password manager AES-256 encryption kullanır ve güvenlidir. Ancak production ortamlarında mümkünse SSH key authentication kullanmanız önerilir. SEND komutu ile config dosyasında şifre saklamak güvenli değildir.

## Ekran Görünümü

```
┌─────────────────────────────────────────────────────────────┐
│ 🔐 SSH Host Manager                            Hosts: 7     │
├─────────────────────────────────────────────────────────────┤
│   ▼ 🔴 Production                                           │
│     ▼ 🌐 Web Servers                                        │
│ ➤       🖥️ Web Server 1                                     │
│         🖥️ Web Server 2                                     │
│     ▶ 🗄️ Database Servers                                   │
│       🖥️ Bastion Host                                       │
│   ▶ 🟡 Staging                                              │
│   ▶ 🟢 Development                                          │
├─────────────────────────────────────────────────────────────┤
│ 🖥️ Web Server 1                                             │
│ Primary web server                                          │
│                                                             │
│ 💻 Command: ssh -t jumphost@bastion "ssh -t deploy@web1..." │
├─────────────────────────────────────────────────────────────┤
│ ↑↓/jk: Navigate  ←→/hl: Collapse/Expand  Enter: Select     │
└─────────────────────────────────────────────────────────────┘
```

## Geliştirme

Projeyi çalıştırmak için:

```bash
go run main.go
```

Build:

```bash
go build -o go-ssh
```

## Bağımlılıklar

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Stil ve görünüm
- [yaml.v3](https://gopkg.in/yaml.v3) - YAML parsing

## Lisans

MIT
