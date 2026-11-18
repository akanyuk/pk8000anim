; ============================================================
; Плеер анимации ПК8000 (Хобби, Веста, Сура) — режим 2, 256x192
; Сгенерировано утилитой pk8000anim; данные анимации — data.asm
;
; Формат дифф-кадров (memsave, адреса — физические в видео-ОЗУ):
;   dw addr     — начальный адрес указателя экрана
;   %00nnnnnn   — COPY  nnnnnn+1 (1..64) байт из потока
;   %01nnnnnn   — SKIP  nnnnnn+1 (1..64) байт указателя
;   %100nnnnn   — FILL  nnnnnn+1 (1..32) байта следующим байтом
;   %101yxxxx   — LONG SKIP: +xxxx*256 (+128 при y=1)
;   %11xxxxxx   — END кадра
;
; Сборка: build.sh (Linux) или build.bat (Windows)
; Загрузка: Alt-F3 ("Загрузить и выполнить"); сброс — выход в Бейсик
; ============================================================

P_MEMCFG  EQU 80h      ; конфигурация страниц памяти
P_SYSCTRL EQU 86h      ; гашение изображения, строб принтера
P_MODE    EQU 84h      ; режим экрана и банк видео-ОЗУ
P_COLORS  EQU 88h      ; цвет рамки в старшем тетраде
P_BUFBASE EQU 91h      ; база буфер экрана (1800h)
P_GRBASE  EQU 92h      ; база массива графики (биты инвертированы!)
P_COLBASE EQU 93h      ; база массива цвета   (биты инвертированы!)

TICKLOOP  EQU 620      ; итераций цикла ожидания на 1 тик (1/100 с)

        ORG 4000h

start:
        DI
        MVI A,0FFh
        OUT P_MEMCFG     ; все страницы -> ОЗУ
        MVI A,90h
        OUT P_SYSCTRL    ; гашение выключено
        MVI A,000h
        OUT P_COLORS     ; старший тетрад — цвет рамки
        MVI A,06h
        OUT P_BUFBASE    ; буфер экрана по адресу 1800h
        MVI A,0FFh
        OUT P_GRBASE     ; массив графики по адресу 0000h
        MVI A,0F7h
        OUT P_COLBASE    ; массив цвета по адресу 2000h

        ; буфер экрана: коды 00h..FFh для каждой из 3 частей
        LXI H,KEY_BUFFER
        LXI D,1800h
        LXI B,0300h
        CALL BLKCP

        ; ключевой кадр (кадр 0)
        LXI H,KEY_GRAPHICS
        LXI D,0000h
        LXI B,1800h
        CALL BLKCP
        LXI H,KEY_COLORS
        LXI D,2000h
        LXI B,1800h
        CALL BLKCP

        MVI A,10h
        OUT P_MODE       ; режим 2 (графика 256x192), банк видео-ОЗУ 0
        MVI A,0FCh
        OUT P_MEMCFG     ; вернуть ПЗУ в страницу 0

; --- главный цикл: задержка кадра -> проигрыш диффа ---
mainloop:
        CALL GETDELAY    ; A = длительность кадра, тиков
        MOV D,A
dloop:  LXI B,TICKLOOP
dwait:  DCX B
        MOV A,B
        ORA C
        JNZ dwait
        DCR D
        JNZ dloop

        CALL PLAY
        JMP mainloop

; A = длительность текущего кадра; таблица DELAY_TAB обходится по кругу
GETDELAY:
DPTR:   LXI H,DELAY_TAB
        MOV B,H
        MOV C,L
        LXI H,DELAY_END
        MOV A,B
        CMP H
        JNZ dnw
        MOV A,C
        CMP L
        JNZ dnw
        LXI H,DELAY_TAB
        SHLD DPTR+1
        MOV B,H
        MOV C,L
dnw:    MOV H,B
        MOV L,C
        MOV A,M
        INX H
        SHLD DPTR+1
        RET

; проигрывает один дифф-кадр (memsave); указатель потока ходит по кругу
PLAY:
PLX:    LXI H,STREAM_START
        MOV B,H
        MOV C,L
        LXI H,STREAM_END
        MOV A,B
        CMP H
        JNZ pnw
        MOV A,C
        CMP L
        JNZ pnw
        LXI H,STREAM_START
        JMP prd
pnw:    MOV H,B
        MOV L,C
prd:    MOV E,M
        INX H
        MOV D,M
        INX H          ; DE = начальный адрес видео-ОЗУ

NEXTCMD:    MOV A,M
        INX H
        STA CMDTMP
        ANI 0C0h
        JZ doCopy
        CPI 40h
        JZ doSkip
        CPI 80h
        JZ doFillLong
        ; конец кадра: запомнить позицию потока для следующего вызова
        MOV A,L
        STA PLX+1
        MOV A,H
        STA PLX+2
        RET

doCopy: LDA CMDTMP
        ANI 3Fh
        MOV B,A
        INR B
c1:     MOV A,M
        STAX D
        INX H
        INX D
        DCR B
        JNZ c1
        JMP NEXTCMD

doFillLong:
        LDA CMDTMP
        ANI 20h
        JNZ doLong
        LDA CMDTMP
        ANI 1Fh
        MOV B,A
        INR B
        MOV A,M
        INX H
f1:     STAX D
        INX D
        DCR B
        JNZ f1
        JMP NEXTCMD

doLong: LDA CMDTMP
        ANI 0Fh
        MOV B,A
        MVI C,0
        XCHG
        DAD B
        XCHG           ; +xxxx*256
        LDA CMDTMP
        ANI 10h
        JZ NEXTCMD
        MVI B,0
        MVI C,80h
        XCHG
        DAD B
        XCHG           ; +128
        JMP NEXTCMD

doSkip: LDA CMDTMP
        ANI 3Fh
        MOV B,A
        INR B
s1:     INX D
        DCR B
        JNZ s1
        JMP NEXTCMD

; копирование BC байт из HL в DE (адреса назначения — в видео-ОЗУ)
BLKCP:  MOV A,M
        STAX D
        INX H
        INX D
        DCX B
        MOV A,B
        ORA C
        JNZ BLKCP
        RET

; заполнение BC байт по DE значением A
FILM:   MOV H,A          ; значение в H: MOV не трогает флаги
fm1:    STAX D
        INX D
        DCX B
        MOV A,B
        ORA C
        MOV A,H
        JNZ fm1
        RET

CMDTMP: DB 0
