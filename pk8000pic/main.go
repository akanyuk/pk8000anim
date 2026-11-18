package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const usage = `pk8000pic — генератор программ ПК8000 (Хобби, Веста, Сура) из PNG/GIF.

Использование:
  pk8000pic gen <изображение.png|gif> [флаги]
      Генерирует проект: исходник программы (8080), скрипты сборки
      build.sh/build.bat и собранный .cas.
      Флаги:
        -o каталог     каталог вывода (по умолчанию рядом с изображением)
        -name имя      имя проекта (по умолчанию из имени файла)
        -border N      цвет рамки 0..15 (по умолчанию 0 — чёрный)
        -stretch       растянуть до 256x192 без сохранения пропорций
        -no-cas        не собирать .cas сразу

  pk8000pic asm <файл.asm> [флаги]
      Собирает исходник ассемблера 8080 в .cas для ПК8000.
      Флаги:
        -o файл.cas    имя выходного файла
        -name имя      имя файла на магнитной ленте (до 6 символов)

Примеры:
  pk8000pic gen kartinka.png -o out
  pk8000pic asm out/kartinka.asm -o out/kartinka.cas
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "gen":
		err = cmdGen(os.Args[2:])
	case "asm":
		err = cmdAsm(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		// `pk8000pic file.png` — сокращение для gen
		if ext := strings.ToLower(filepath.Ext(os.Args[1])); ext == ".png" || ext == ".gif" {
			err = cmdGen(os.Args[1:])
		} else {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}
}

// reorderArgs позволяет писать флаги после имени файла:
// `gen image.png -o out` превращается в `gen -o out image.png`.
func reorderArgs(args []string) []string {
	if len(args) > 1 && !strings.HasPrefix(args[0], "-") {
		out := append([]string{}, args[1:]...)
		return append(out, args[0])
	}
	return args
}

func cmdGen(args []string) error {
	args = reorderArgs(args)
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	outDir := fs.String("o", "", "каталог вывода")
	name := fs.String("name", "", "имя проекта")
	border := fs.Int("border", 0, "цвет рамки 0..15")
	stretch := fs.Bool("stretch", false, "растянуть до 256x192")
	noCAS := fs.Bool("no-cas", false, "не собирать .cas")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("укажите ровно одно изображение (PNG или GIF)")
	}
	if *border < 0 || *border > 15 {
		return fmt.Errorf("цвет рамки должен быть в диапазоне 0..15")
	}
	imgPath := fs.Arg(0)
	img, err := loadImage(imgPath)
	if err != nil {
		return err
	}
	proj := *name
	if proj == "" {
		proj = strings.TrimSuffix(filepath.Base(imgPath), filepath.Ext(imgPath))
	}
	safe := sanitizeName(proj)
	dir := *outDir
	if dir == "" {
		dir = filepath.Dir(imgPath)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	fmt.Printf("Исходное изображение: %s\n", imgPath)
	rgba := fitTo256x192(img, *stretch)
	sd := encodeScreen(rgba)
	fmt.Printf("Кадр 256x192 закодирован: графика %d байт, цвет %d байт\n",
		len(sd.Graphics), len(sd.Colors))

	asmName := safe + ".asm"
	casName := safe + ".cas"
	src := emitASM(filepath.Base(imgPath), sd, byte(*border))
	asmPath := filepath.Join(dir, asmName)
	if err := os.WriteFile(asmPath, []byte(src), 0o644); err != nil {
		return err
	}
	fmt.Printf("Исходник программы: %s\n", asmPath)

	shellCmd, batCmd := asmInvocation()
	sh, bat := emitBuildScripts(asmName, casName, shellCmd, batCmd)
	shPath := filepath.Join(dir, "build.sh")
	batPath := filepath.Join(dir, "build.bat")
	if err := os.WriteFile(shPath, []byte(sh), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(batPath, []byte(bat), 0o644); err != nil {
		return err
	}
	fmt.Printf("Скрипты сборки: %s, %s\n", shPath, batPath)

	if !*noCAS {
		if err := buildFromFile(asmPath, filepath.Join(dir, casName), safe); err != nil {
			return err
		}
		fmt.Printf("Готовая программа: %s\n", filepath.Join(dir, casName))
	}
	return nil
}

func cmdAsm(args []string) error {
	args = reorderArgs(args)
	fs := flag.NewFlagSet("asm", flag.ExitOnError)
	out := fs.String("o", "", "выходной .cas")
	name := fs.String("name", "", "имя файла на ленте")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("укажите исходный файл .asm")
	}
	asmPath := fs.Arg(0)
	outPath := *out
	if outPath == "" {
		outPath = strings.TrimSuffix(asmPath, filepath.Ext(asmPath)) + ".cas"
	}
	proj := *name
	if proj == "" {
		proj = filepath.Base(strings.TrimSuffix(asmPath, filepath.Ext(asmPath)))
	}
	if err := buildFromFile(asmPath, outPath, proj); err != nil {
		return err
	}
	fmt.Printf("Готово: %s\n", outPath)
	return nil
}

func buildFromFile(asmPath, outPath, name string) error {
	src, err := os.ReadFile(asmPath)
	if err != nil {
		return err
	}
	base, code, err := Assemble(string(src))
	if err != nil {
		return fmt.Errorf("%s: %w", asmPath, err)
	}
	if len(code) == 0 {
		return fmt.Errorf("%s: пустой результат ассемблирования", asmPath)
	}
	if base < 0 || base+len(code) > 0x10000 {
		return fmt.Errorf("%s: программа выходит за пределы 64 Кб", asmPath)
	}
	cas := buildCAS(name, base, base, code)
	if err := os.WriteFile(outPath, cas, 0o644); err != nil {
		return err
	}
	fmt.Printf("Ассемблировано: %d байт по адресу %04Xh\n", len(code), base)
	return nil
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '.' {
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "picture"
	}
	return out
}
