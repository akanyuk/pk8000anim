# sura_ap

Инструменты для советского бытового ПК **ПК8000** («Сура», «Веста», «Хобби»):

- **`pk8000pic/`** — генератор программ на Go: PNG/GIF → программа ПК8000,
  выводящая изображение на экран (режим 2, 256×192). Отдаёт исходник на
  ассемблере КР580ВМ80А, скрипты сборки (Windows/Linux) и готовый `.cas`.
  Ассемблер встроен, внешних компиляторов не требуется — только Go.
- **`emu80-mcp/`** — эмулятор Emu80 с включённым MCP-сервером (автозагрузка
  `.cas`, скриншоты экрана PNG, чтение/запись памяти и портов по HTTP) —
  для автоматических тестов программ ПК8000.
- **`docs/PK8000_stroenie_ekrana.md`** — справочник по видеосистеме ПК8000:
  три режима экрана, карты видео-ОЗУ, регистры, палитра.

## pk8000pic

```sh
cd pk8000pic
go build -o pk8000pic .

# собрать программу из картинки
./pk8000pic gen testdata/example.png -o out -name example
# -> out/example.asm, out/build.sh, out/build.bat, out/example.cas

# пересобрать исходник (то же делают build.sh / build.bat)
./pk8000pic asm out/example.asm -o out/example.cas
```

`.cas` загружается в эмуляторе ПК8000 через Alt-F3 («Загрузить и выполнить»)
или перетаскиванием файла в окно; на реальной машине — командой
`BLOAD"имя",R` с кассеты.

Флаги `gen`: `-o каталог`, `-name имя` (до 6 символов), `-border 0..15`
(цвет рамки), `-stretch` (растянуть без сохранения пропорций), `-no-cas`.

## emu80-mcp

```sh
cd emu80-mcp
./run.sh            # запуск ПК8000 с MCP-сервером
```

MCP-сервер: `http://127.0.0.1:19781/mcp` (JSON-RPC over HTTP).
Пример вызова (загрузка и скриншот):

```sh
curl -s -X POST http://127.0.0.1:19781/mcp -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"emu_load","arguments":{"file":".../example.cas","autorun":true}}}'
curl -s -X POST http://127.0.0.1:19781/mcp -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"screen","arguments":{}}}'
```

Основные инструменты: `emu_load`, `emu_run_for`, `emu_reset`, `screen`
(base64-PNG), `mem_read`/`mem_write`, `port_read`/`port_write`, `disasm`,
`cpu_state`, `kbd_text`. Строки в аргументах парсятся как шестнадцатеричные
(«255» = 0x255) — для десятичных значений передавайте JSON-числа.

Важно: конфигурация платформы передаётся **только** флагом `--conf-file`
(иначе эмулятор показывает модальный диалог выбора платформы до старта MCP).

Сборка эмулятора из исходников (если понадобится пересобрать):
`git clone https://github.com/vpyk/emu80v4 && cd emu80v4 && mkdir build && cd build
&& qmake6 MCP_SERVER=1 ../src/Emu80qt.pro && make`
(нужны Qt6 с модулями multimedia/widgets/openglwidgets).

## Документация

`docs/PK8000_stroenie_ekrana.md` — структура экрана ПК8000 по книгам
«Сура/Веста/Хобби ПК8000» (Книга 2, приложения 3, 4, 6) с сайта
[pk8000.narod.ru](http://pk8000.narod.ru/): режимы 0/1/2, раскладка
видео-ОЗУ с адресами, регистры 84h/88h/90h–93h, ячейки FA05–FA12, палитра.
