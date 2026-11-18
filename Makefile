# Сборка pk8000anim для Linux, Windows и macOS.
# Требуется Go; кросс-сборка идёт штатными средствами go build (GOOS/GOARCH).
# Результаты — в dist/ (каталог в .gitignore, в репозиторий не попадают).
#
#   make            - все платформы (linux, windows, macos)
#   make linux      - linux/amd64
#   make windows    - windows/amd64
#   make macos      - darwin/arm64 и darwin/amd64
#   make build      - бинарник для текущей системы
#   make clean      - удалить dist/

BINARY  := pk8000anim
DIST    := dist
LDFLAGS := -s -w

.PHONY: all build linux windows macos clean

all: linux windows macos

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) .

linux:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-amd64 .

windows:
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-windows-amd64.exe .

macos:
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-darwin-amd64 .

clean:
	rm -rf $(DIST)
