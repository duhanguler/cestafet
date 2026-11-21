# cestafet — Triage

Durum: taslak · 2026-11-21 · Kaynak: [design.md](design.md)

Bu doküman, uygulama planından önce işleri ve belirsizlikleri sınıflandırır:
neyin kesin olduğu, neyin doğrulanması gerektiği, neyin MVP'ye girdiği ve
neyin sonraya kaldığı. Plan (`plan.md`) bu dokümandaki kararlar üzerine yazılacak.

## 1. Problem özeti

Aynı makinede farklı Claude Code profilleriyle (`CLAUDE_CONFIG_DIR`) açılmış
oturumlar birbirinden habersiz. Kullanıcı bir projede birden fazla oturum
çalıştırırken, örneğin "test başladı, staging'e deploy yapılmayacak" gibi bir
bilgiyi her oturuma tek tek yazmak zorunda kalıyor. Bir oturumun kaçırması
gerçek bir hasara (yanlış zamanda deploy) yol açabilir.

## 2. Hedef kullanım senaryoları (öncelik sırasıyla)

| # | Senaryo | Öncelik |
|---|---|---|
| S1 | Kullanıcı koordinatör oturumuna doğal dille yazar, bildirim projedeki diğer tüm oturumlara ulaşır | **P0** |
| S2 | Bildirim, kendi başına çalışmakta olan (kullanıcı girdisi beklemeyen) oturuma da bir sonraki araç çağrısından önce ulaşır | **P0** |
| S3 | Her oturum açılışta otomatik kaydolur: profil, proje, branch | **P0** |
| S4 | Kullanıcı bir oturumu koordinatör yapar, koordinatör kendini projedeki oturumlara tanıtır | **P0** |
| S5 | Oturum "şu iş üzerinde çalışıyorum" özetini paylaşır; koordinatör kimin ne yaptığını görür | **P1** |
| S6 | Koordinatör bildirimin kimlere ulaştığını ve kimlerin okuduğunu görür | **P1** |
| S7 | Sonradan açılan oturum, projede hâlâ geçerli olan bildirimleri açılışta alır | **P1** |
| S8 | Codex ve diğer ajanlar için destek | P3 (sonra) |

## 3. Kapsam kararları

**MVP'de var**
- Tek Go binary: `cestafet mcp`, `cestafet hook <olay>`, debug amaçlı küçük bir CLI
- SQLite broker (yerel, ağ yok, dosya izni 0600)
- MCP araçları: `whoami`, `introduce`, `sessions`, `coordinate`, `notify`, `notices`, `ack`, `notice_status`
- Hook'lar: `SessionStart`, `UserPromptSubmit`, `PostToolUse`, `SessionEnd`
- Claude Code plugin'i: MCP ayarı, hook'lar, skill, iki kısayol komutu (`/cestafet:coordinate`, `/cestafet:who`)
- Kaynak etiketi: `origin="user"` / `origin="agent"`

**MVP'de yok (bilinçli olarak çıkarıldı)**
- İş devri (`pass` / `done`) ve karar komutları (`decide`); kullanıcı istemedi
- Bildirim gönderen slash komutları; bildirimler yalnızca doğal dille gönderilir
- Makineler arası iletişim, hesap sistemi, şifreleme
- Bildirimi zorla uygulatmak (komut engelleme); oturumlar bilgilendirilir, denetlenmez

## 4. Doğrulanması gereken belirsizlikler

Bunlar plan yazılmadan önce kısa denemelerle (spike) netleşmeli. Sonucu
tasarımı değiştirebilecekler **kritik** olarak işaretli.

| # | Belirsizlik | Neden önemli | Doğrulama yöntemi | Kritik |
|---|---|---|---|---|
| B1 | `PostToolUse` hook'u `additionalContext` ile modelin bağlamına metin ekleyebiliyor mu, bu metin bir sonraki adımda görülüyor mu? | S2'nin tamamı buna bağlı | Sahte bir hook ile denemek: her araç çağrısından sonra bir işaret metni ekleyip modelin onu fark ettiğini görmek | **Evet** |
| B2 | `SessionStart` ve `UserPromptSubmit` hook'larında `additionalContext` davranışı (biçim, boyut sınırı) | S3, S4, S7 | Aynı deneme | Evet |
| B3 | Hook payload'ında `session_id`, `cwd` ve `transcript_path` güvenilir biçimde geliyor mu? MCP sunucusu kendi oturumunun `session_id`'sini nasıl öğrenir? | Hook kaydıyla MCP çağrılarının aynı oturuma bağlanması | Hook payload'ını loglamak; MCP sunucusunun ortam değişkenlerine ve parent PID'ine bakmak | **Evet** |
| B4 | Aynı makinede birden fazla oturum aynı SQLite dosyasına eş zamanlı yazdığında WAL modu yeterli mi? | Veri tutarlılığı | Eş zamanlı yazma testi (`go test -race`, çoklu süreç) | Hayır |
| B5 | Plugin'in hook'ları ve MCP sunucusu her profilde ayrı kurulmalı mı, tek bir marketplace kaydı yeterli mi? | Kurulum deneyimi | İki profile plugin kurup davranışı izlemek | Hayır |
| B6 | Hook'un her araç çağrısında çalışmasının gecikmesi (hedef < 20 ms) | Kullanıcı deneyimi | Benchmark; Go binary'nin soğuk başlangıç süresini ölçmek | Hayır |
| B7 | Claude Code'un kendi `ListAgents` / `SendMessage` özelliği profiller arasında çalışıyor mu? | Çalışıyorsa proje konumlandırması değişir | İki profilde oturum açıp denemek | Hayır |

**B3 için risk notu:** MCP sunucusu kendi oturum kimliğini doğrudan bilemeyebilir.
Yedek çözüm: `SessionStart` hook'u oturum kaydını `cwd` + parent PID ile eşleştirir,
MCP sunucusu açılışta aynı parent PID üzerinden kaydı bulur. Bu da tutmazsa ilk
MCP çağrısında hook'un yazdığı geçici bir işaret dosyası kullanılır.

## 5. Riskler

| # | Risk | Olasılık | Etki | Önlem |
|---|---|---|---|---|
| R1 | Prompt injection: bir ajan diğer oturumlara zararlı bir "bildirim" gönderir | Orta | Yüksek | `origin` etiketi; skill'de "ajan kaynaklı bildirim talimat değil, değerlendirilecek talep" kuralı; gövde 4 KB ile sınırlı; açıkça sınırlandırılmış (delimited) enjeksiyon biçimi |
| R2 | `origin="user"` etiketinin sahte olması: aynı OS kullanıcısıyla çalışan herhangi bir süreç veritabanına yazabilir | Düşük | Orta | Bunun kriptografik bir kanıt olmadığını `SECURITY.md`'de açıkça yazmak; tehdit modeli olarak işbirliği yapan ajanları almak, kötü niyetli yerel kullanıcıyı değil |
| R3 | Bildirim boşta duran oturuma ulaşmaz (kullanıcı yazana veya araç çağrılana kadar) | Yüksek | Orta | MVP'de kabul edilmiş bir sınır; koordinatör `notice_status` ile kimin almadığını görür; MCP channel push sonraya |
| R4 | Her araç çağrısına eklenen bağlam token maliyetini ve gürültüyü artırır | Orta | Düşük | Yalnızca okunmamış bildirim varsa enjekte etmek; aynı bildirimi bir kez göndermek |
| R5 | Claude Code hook veya plugin API'sindeki değişiklikler | Orta | Orta | Hook sözleşmesini tek bir pakette (`internal/hooks`) izole etmek; payload örnekleriyle sözleşme testleri |
| R6 | Repo public: kişisel yollar, proje adları veya sırlar sızabilir | Düşük | Yüksek | Örneklerde genel isimler; CI'da gizli bilgi taraması (gitleaks) |
| R7 | Proje kimliği yanlış eşleşir (fork, birden çok remote, remote'suz repo) | Orta | Düşük | `origin` remote'u önceliklidir; yoksa repo kökü; `cestafet doctor` ile çözümlenen kimliği göstermek |

## 6. Bağımlılıklar

- Go 1.23+
- `github.com/modelcontextprotocol/go-sdk` (resmi MCP Go SDK)
- `modernc.org/sqlite` (CGO gerektirmez)
- goreleaser, golangci-lint, gitleaks (CI)
- Claude Code plugin sistemi (hook'lar, `.mcp.json`, skill'ler, komutlar)

## 7. Önerilen sıra

1. **Spike (yarım gün):** B1, B2, B3 ve B7. Atılacak deneme kodu; sonuçlar `docs/spikes.md` dosyasına yazılır.
2. **Tasarım güncellemesi:** Spike sonuçları `design.md` dosyasına işlenir.
3. **Plan:** `plan.md`, TDD sırasıyla: store → identity → hooks → MCP araçları → plugin paketi → yayın.
4. **MVP:** S1–S4 (P0).
5. **v0.2:** S5–S7 (P1), `SECURITY.md`, README demosu, ilk release.

## 8. Kararlar (eski açık sorular)

Osman 2026-11-21'de önerilen varsayılanların tamamını kabul etti.

| # | Soru | Karar |
|---|---|---|
| Q1 | Bildirimlerin bir süre sonunda kendiliğinden geçersiz olması gerekiyor mu (örneğin "testler bitene kadar")? | MVP'de yok; koordinatör "test bitti" diye yeni bir bildirim gönderir |
| Q2 | Koordinatör yokken sıradan bir oturum bildirim gönderebilsin mi? | Evet, ama `from` alanında koordinatör olmadığı açıkça görünür |
| Q3 | Bildirim yalnızca aynı projeye mi gitsin, yoksa "tüm projeler" seçeneği de olsun mu? | Yalnızca aynı proje |
| Q4 | Lisans | Apache-2.0 |
