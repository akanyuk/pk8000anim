@echo off
rem Сборка программы ПК8000: example.asm -> example.cas
rem Утилита-ассемблер pk8000pic (Go). Переопределить путь: set PK8000PIC=...
cd /d "%~dp0"

if not "%PK8000PIC%"=="" (
    call %PK8000PIC% asm example.asm -o example.cas
    goto :done
)
where pk8000pic >nul 2>nul && (
    call pk8000pic asm example.asm -o example.cas
    goto :done
)
pk8000pic asm example.asm -o example.cas
:done
if errorlevel 1 (
    echo Ошибка сборки
    exit /b 1
)
echo Готово: example.cas (загрузить в эмуляторе ПК8000: Alt-F3)
