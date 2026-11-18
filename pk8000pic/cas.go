package main

import (
	"bytes"
	"fmt"
	"strings"
)

// Формат двоичного .cas ПК8000 (как в файлах с pk8000.narod.ru):
//
//	8 байт  сигнатура 1F A6 DE BA CC 13 7D 74
//	10 байт маркер типа D0h (двоичный файл)
//	6 байт  имя файла (прописными, дополнено пробелами)
//	8 байт  сигнатура ещё раз
//	2 байта адрес загрузки (LE)
//	2 байта адрес конца, невключительно = загрузка + длина (LE)
//	2 байта адрес запуска (LE)
//	...     данные
var casSignature = []byte{0x1F, 0xA6, 0xDE, 0xBA, 0xCC, 0x13, 0x7D, 0x74}

func buildCAS(name string, loadAddr, runAddr int, code []byte) []byte {
	var buf bytes.Buffer
	buf.Write(casSignature)
	buf.Write(bytes.Repeat([]byte{0xD0}, 10))
	buf.WriteString(casName(name))
	buf.Write(casSignature)
	put16 := func(v int) {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
	}
	put16(loadAddr)
	put16(loadAddr + len(code))
	put16(runAddr)
	buf.Write(code)
	return buf.Bytes()
}

func casName(name string) string {
	n := strings.ToUpper(name)
	var b strings.Builder
	for _, r := range n {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		s = "PIC"
	}
	if len(s) > 6 {
		s = s[:6]
	}
	return fmt.Sprintf("%-6s", s)
}
