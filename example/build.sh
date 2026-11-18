#!/bin/sh
# Сборка программы ПК8000: example.asm -> example.cas
# Утилита-ассемблер pk8000pic (Go). Переопределить путь: PK8000PIC=...
set -e
cd "$(dirname "$0")"

pic() {
    if [ -n "${PK8000PIC:-}" ]; then "$PK8000PIC" "$@"; return; fi
    if command -v pk8000pic >/dev/null 2>&1; then pk8000pic "$@"; return; fi
    /home/marinov/Downloads/pk8000pic/pk8000pic "$@"
}

pic asm example.asm -o example.cas
echo "Готово: example.cas (загрузить в эмуляторе ПК8000: Alt-F3)"
