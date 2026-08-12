# RIKA collector w Go

Aplikacja odwzorowuje dane zbierane przez załączone skrypty Bash i łączy się z płytką bezpośrednio przez SSH na porcie 22.

## Zbierane dane

- `redis_network_info.txt` z klucza Redis `redis-shared-data:NetworkInfoData:NetworkInfo`;
- pełne drzewo `/data/profiles`;
- regularne pliki `*.dlog` bezpośrednio z `/data/log`;
- wyniki `cat /etc/version`, `date`, `uname -a`, `ifconfig`, `ip a`, `ip route` i `mount`;
- pełne drzewo `/etc/NetworkManager/system-connections`;
- dump NetworkManager dla interfejsu `eth0`.

Program kontynuuje pracę po błędzie pojedynczej sekcji. Status znajduje się w `collection_summary.txt`. Na końcu tworzone jest archiwum `rika_files_YYYYMMDD_HHMMSS.tgz`.

## Konfiguracja

Program przyjmuje dokładnie jeden argument: ścieżkę do pliku konfiguracyjnego.

```text
IP=192.168.222.2
USER=root
PASSWRD=!C0vF3F3
DLOG_DAYS=3
```

`USER` musi mieć wartość `root`. Nie są obsługiwane dodatkowe klucze konfiguracyjne.

## dlogparser

Na Linuksie plik wykonywalny `dlogparser` musi znajdować się w tym samym katalogu co `rika-collector`. Po udanej konwersji źródłowy plik `.dlog` jest usuwany.

Na Windows konwersja `.dlog` do `.csv` jest pomijana, a pobrane pliki `.dlog` pozostają w paczce wynikowej.

## Budowanie w Dockerze

Obraz bazuje na Fedora 41. Ponieważ Fedora 41 jest wydaniem EOL, Dockerfile używa jej archiwalnych repozytoriów. W obrazie instalowany jest Go 1.26.5, a suma SHA-256 archiwum Go jest sprawdzana. Wersja modułu `golang.org/x/crypto` jest przypięta w `go.mod` i `go.sum`.

```bash
make docker
```

Osobne kroki są również dostępne jako `make docker-build` i `make docker-release`.

Wyniki są dostępne bezpośrednio na hoście:

```text
dist/
├── fedora41-amd64/
│   ├── collector.conf
│   ├── dlogparser              # tylko gdy był w katalogu projektu podczas docker build
│   └── rika-collector
└── windows11-amd64/
    ├── collector.conf
    ├── rika-collector-win11.bat
    └── rika-collector.exe
```

Jeżeli `dlogparser` ma zostać automatycznie skopiowany do wyniku Linux, umieść go w katalogu projektu przed `make docker-build`.

Równoważne polecenia Docker bez użycia celu `docker-release`:

```bash
docker build --pull -t rika-collector-builder .
mkdir -p dist
docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp \
    -v "$PWD/dist:/workspace/dist:Z" \
    rika-collector-builder
```

## Uruchomienie

Fedora 41:

```bash
cd dist/fedora41-amd64
./rika-collector collector.conf
```

Windows 11, PowerShell:

```powershell
cd dist\windows11-amd64
.\rika-collector.exe collector.conf
```

Wyniki powstają w katalogu `collected_files` obok uruchomionego programu.

## Kody wyjścia

- `0` — wszystkie sekcje zakończyły się powodzeniem;
- `1` — błąd konfiguracji, połączenia SSH albo tworzenia archiwum;
- `2` — co najmniej jedna sekcja zbierania danych zakończyła się błędem, ale archiwum zostało utworzone.

## Bezpieczeństwo

W pliku konfiguracyjnym hasło jest przechowywane jako zwykły tekst. Katalog `networkmanager_system_connections` może zawierać hasła, klucze prywatne lub inne dane uwierzytelniające. Plik konfiguracji i wygenerowane archiwum należy przechowywać i przesyłać w bezpieczny sposób.
