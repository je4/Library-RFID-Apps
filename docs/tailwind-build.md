### Tailwind CSS Build-Anleitung

Dieses Dokument beschreibt die Erzeugung und Aktualisierung von `data/web/static/css/tailwind.min.css` für das Library-RFID-Apps Webinterface.

---

### 1. Konzept: JIT vs. Vollständiger CSS-Build

Tailwind CSS (ab Version 4) basiert auf einer hochoptimierten **Just-In-Time (JIT) Engine**. Anstelle einer monolithischen, viele Megabyte großen CSS-Datei mit allen theoretisch denkbaren Klassen- und Farb-Kombinationen scannt der Compiler gezielt die verwendeten Template- und JavaScript-Dateien. 

Dadurch wird ein maximaler Gesamtnutzen erzielt:
- **Minimale Bundle-Größe (~25 KB minifiziert):** Extrem schnelle Ladezeiten im Browser und geringe Latenz bei RFID- und Kiosk-Stationen.
- **Vollständige Flexibilität:** Jede in GoHTML oder JS genutzte Klasse (inklusive aller Responsive-Präfixe wie `sm:grid-cols-3`, `lg:gap-6`, etc.) wird automatisch und fehlerfrei in das finale CSS kompiliert.

---

### 2. Eingabedatei: `data/web/static/css/tailwind-input.css`

Die Konfiguration und Quellpfade werden über CSS-Direktiven gesteuert:

```css
@import "tailwindcss";

/* Quellpfade für das automatische Scannen aller Klassen */
@source "../../templates/**/*.gohtml";
@source "../js/**/*.js";
```

---

### 3. Befehle zum Erzeugen von `tailwind.min.css`

#### Voraussetzungen
- Node.js (>= 18) installiert.

#### Option A: Direkte Ausführung via `npx` (Empfohlen, ohne globale Installation)

Im Projekt-Wurzelverzeichnis (`Library-RFID-Apps`):

```powershell
# 1. Einmalige / Initiale Installation im lokalen Projekt oder global:
npm install -D tailwindcss @tailwindcss/cli

# 2. Kompilieren und Minifizieren von tailwind.min.css:
npx @tailwindcss/cli -i data/web/static/css/tailwind-input.css -o data/web/static/css/tailwind.min.css --minify
```

#### Option B: Automatischer Watch-Modus während der Entwicklung

Um CSS-Klassen bei Änderungen an GoHTML-Templates automatisch im Hintergrund neu zu bündeln:

```powershell
npx @tailwindcss/cli -i data/web/static/css/tailwind-input.css -o data/web/static/css/tailwind.min.css --watch
```

#### Option C: Standalone Tailwind CLI (Ohne Node.js / npm)

Tailwind Labs stellt eigenständige Binaries für Windows bereit:

```powershell
# Download der Standalone-CLI (Windows x64):
Invoke-WebRequest -Uri "https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-windows-x64.exe" -OutFile "tailwindcss.exe"

# Erzeugen des CSS-Bundles:
.\tailwindcss.exe -i data/web/static/css/tailwind-input.css -o data/web/static/css/tailwind.min.css --minify
```

---

### 4. Einbindung in den Go-Build-Prozess

Da die statischen Dateien über `embed.FS` in `library-desk.exe` einkompiliert werden, wird nach der CSS-Generierung das Go-Binary neu gebaut:

```powershell
go build -o library-desk.exe ./cmd/library-desk
```
