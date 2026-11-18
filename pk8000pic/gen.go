package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// emitASM формирует исходник программы ПК8000 (8080) с данными изображения.
func emitASM(imgName string, sd ScreenData, border byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, `; ============================================================
; Вывод изображения на экран ПК8000 (Хобби, Веста, Сура)
; Сгенерировано утилитой pk8000pic из %s
;
; Режим экрана 2: 256x192 точек; для каждого отрезка из 8 точек
; заданы свой цвет изображения, фона и битовая маска.
; Структура видео-ОЗУ (окно 0000h..3FFFh):
;   0000h-17FFh  массив графики   (3 x 2048 байт)
;   1800h-1AFFh  буфер экрана     (3 x 256 кодов знаков)
;   2000h-37FFh  массив цвета     (3 x 2048 байт)
;
; Сборка: build.sh (Linux) или build.bat (Windows)
; Загрузка в эмулятор: Alt-F3 ("Загрузить и выполнить")
; ============================================================

; --- порты ввода-вывода (см. Книга 2, Приложение 3) ---
P_MEMCFG   EQU 80h      ; конфигурация страниц памяти
P_SYSCTRL  EQU 86h      ; гашение изображения, строб принтера
P_MODE     EQU 84h      ; режим экрана и банк видео-ОЗУ
P_COLORS   EQU 88h      ; цвет рамки в старшем тетраде
P_BUFBASE  EQU 91h      ; база буфера экрана (режим 2)
P_GRBASE   EQU 92h      ; база массива графики (разряды инвертированы!)
P_COLBASE  EQU 93h      ; база массива цвета   (разряды инвертированы!)

        ORG 4000h

start:
        DI
        MVI A,0FFh
        OUT P_MEMCFG     ; все страницы -> ОЗУ, видео-ОЗУ доступно и на чтение

        MVI A,90h
        OUT P_SYSCTRL    ; гашение выключено, строб принтера в 1

        MVI A,0%02Xh
        OUT P_COLORS     ; младший тетрад - цвет текста, старший - цвет рамки

        MVI A,06h
        OUT P_BUFBASE    ; буфер экрана по адресу 1800h (1800h SHR 10)

        MVI A,0FFh
        OUT P_GRBASE     ; массив графики по адресу 0000h
        MVI A,0F7h
        OUT P_COLBASE    ; массив цвета по адресу 2000h

        LXI H,GRDATA
        LXI D,0000h
        LXI B,1800h
        CALL BLKCP       ; массив графики

        LXI H,SCDATA
        LXI D,1800h
        LXI B,0300h
        CALL BLKCP       ; буфер экрана

        LXI H,CODATA
        LXI D,2000h
        LXI B,1800h
        CALL BLKCP       ; массив цвета

        MVI A,10h
        OUT P_MODE       ; режим 2 (графика 256x192), банк видео-ОЗУ 0

        MVI A,0FCh
        OUT P_MEMCFG     ; вернуть ПЗУ в страницу 0
        ; прерывания не разрешаются: обработчик ПЗУ может переписать
        ; регистры экрана, а изображение должно оставаться на экране

forever:
        JMP forever      ; сброс вернёт машину в Бейсик

; Копирование BC байт из HL в DE (адреса назначения - в видео-ОЗУ)
BLKCP:
        MOV A,M
        STAX D
        INX H
        INX D
        DCX B
        MOV A,B
        ORA C
        JNZ BLKCP
        RET

; --- массив графики: 3 части экрана по 2048 байт ---
GRDATA:
`, imgName, border)

	writeDB(&b, sd.Graphics[:])
	b.WriteString(`
; --- буфер экрана: коды 00h..FFh для каждой из 3 частей ---
SCDATA:
`)
	writeDB(&b, sd.Buffer[:])
	b.WriteString(`
; --- массив цвета: цвет изображения (мл. тетрад) и фона (ст. тетрад) ---
CODATA:
`)
	writeDB(&b, sd.Colors[:])
	b.WriteString("\n        END\n")
	return b.String()
}

func writeDB(b *strings.Builder, data []byte) {
	for i := 0; i < len(data); i += 16 {
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		var parts []string
		for _, v := range data[i:end] {
			parts = append(parts, fmt.Sprintf("0%02Xh", v))
		}
		fmt.Fprintf(b, "        DB %s\n", strings.Join(parts, ","))
	}
}

// asmInvocation — как build-скриптам вызвать утилиту.
// Для скрипта «родной» ОС подставляется реальный путь к текущему бинарнику
// (или go run из каталога модуля); для чужой ОС — утилита из PATH.
func asmInvocation() (shellCmd, batCmd string) {
	shellCmd, batCmd = "pk8000pic", "pk8000pic"
	own := func(shell, bat string) {
		if runtime.GOOS == "windows" {
			batCmd = bat
		} else {
			shellCmd = shell
		}
	}
	if exe, err := os.Executable(); err == nil {
		if base := filepath.Base(exe); base == "pk8000pic" || base == "pk8000pic.exe" {
			own(shellQuote(exe), quoteBAT(exe))
			return shellCmd, batCmd
		}
	}
	if dir := findModuleDir(); dir != "" {
		own("go run "+shellQuote(dir), "go run "+quoteBAT(dir))
	}
	return shellCmd, batCmd
}

func findModuleDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func shellQuote(s string) string {
	if strings.ContainsAny(s, " \t\"'$") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

func quoteBAT(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func emitBuildScripts(asmName, casName, shellCmd, batCmd string) (sh string, bat string) {
	sh = fmt.Sprintf(`#!/bin/sh
# Сборка программы ПК8000: %s -> %s
# Утилита-ассемблер pk8000pic (Go). Переопределить путь: PK8000PIC=...
set -e
cd "$(dirname "$0")"

pic() {
    if [ -n "${PK8000PIC:-}" ]; then "$PK8000PIC" "$@"; return; fi
    if command -v pk8000pic >/dev/null 2>&1; then pk8000pic "$@"; return; fi
    %s "$@"
}

pic asm %s -o %s
echo "Готово: %s (загрузить в эмуляторе ПК8000: Alt-F3)"
`, asmName, casName, shellCmd, asmName, casName, casName)

	bat = fmt.Sprintf(`@echo off
rem Сборка программы ПК8000: %s -> %s
rem Утилита-ассемблер pk8000pic (Go). Переопределить путь: set PK8000PIC=...
cd /d "%%~dp0"

if not "%%PK8000PIC%%"=="" (
    call %%PK8000PIC%% asm %s -o %s
    goto :done
)
where pk8000pic >nul 2>nul && (
    call pk8000pic asm %s -o %s
    goto :done
)
%s asm %s -o %s
:done
if errorlevel 1 (
    echo Ошибка сборки
    exit /b 1
)
echo Готово: %s (загрузить в эмуляторе ПК8000: Alt-F3)
`, asmName, casName, asmName, casName, asmName, casName, batCmd, asmName, casName, casName)

	return sh, bat
}
