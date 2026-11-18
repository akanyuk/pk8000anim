@echo off
rem Сборка анимации ПК8000: player.asm + data.asm -> example.cas
rem Утилита-ассемблер pk8000anim (Go). Переопределить путь: set PK8000ANIM=...
cd /d "%~dp0"

if not "%PK8000ANIM%"=="" (
    call %PK8000ANIM% asm player.asm data.asm -o example.cas
    goto :done
)
where pk8000anim >nul 2>nul && (
    call pk8000anim asm player.asm data.asm -o example.cas
    goto :done
)
pk8000anim asm player.asm data.asm -o example.cas
:done
if errorlevel 1 (
    echo Ошибка сборки
    exit /b 1
)
echo Готово: example.cas (загрузить в эмуляторе ПК8000: Alt-F3)
