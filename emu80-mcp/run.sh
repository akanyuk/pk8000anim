#!/bin/sh
# Запуск эмулятора ПК8000 с MCP-сервером (порт 19781).
cd "$(dirname "$0")"
exec ./Emu80qt-mcp --conf-file pk8000/pk8000.conf "$@"
