package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/image/font/sfnt"
)

// TestFontCoversSourceStrings 檢查專案裡所有字串常值的字元都在內嵌字型中，
// 避免介面出現方框（缺字）。缺字時執行 tools/fontsubset/subset.py 重新產生字型。
func TestFontCoversSourceStrings(t *testing.T) {
	f, err := sfnt.Parse(fontData)
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	missing := map[rune][]string{}

	root := filepath.Join("..", "..")
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// cmd/ 是開發工具，文字只輸出到終端機，不會畫在畫面上。
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "dist" || d.Name() == "previews" || d.Name() == "cmd") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			for _, r := range s {
				if r < 0x80 || unicode.IsControl(r) || unicode.IsSpace(r) {
					continue
				}
				if gi, err := f.GlyphIndex(&buf, r); err != nil || gi == 0 {
					missing[r] = append(missing[r], path)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for r, paths := range missing {
		t.Errorf("字型缺少 %q（U+%04X），用於 %s", r, r, paths[0])
	}
}
