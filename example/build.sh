#!/bin/sh
# Сборка анимации ПК8000: player.asm + data.asm -> example.cas
# Утилита-ассемблер pk8000anim (Go). Переопределить путь: PK8000ANIM=...
set -e
cd "$(dirname "$0")"

pic() {
    if [ -n "${PK8000ANIM:-}" ]; then "$PK8000ANIM" "$@"; return; fi
    if command -v pk8000anim >/dev/null 2>&1; then pk8000anim "$@"; return; fi
    /home/marinov/go/src/github.com/akanyuk/pk8000anim/pk8000anim "$@"
}

pic asm player.asm data.asm -o example.cas
echo "Готово: example.cas (загрузить в эмуляторе ПК8000: Alt-F3)"
