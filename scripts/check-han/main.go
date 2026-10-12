// Command check-han liệt kê chữ Hán còn sót trong bản Việt hoá.
//
// Quét hai nhóm:
//   - string literal trong mã Go (bỏ comment, *_test.go, testdata/, thư mục zh/);
//   - tài nguyên tiếng Việt: assets/prompts/vi, assets/references/vi, assets/styles/vi.
//
// Literal là tham số trực tiếp của lời gọi log (slog.Info, logger.Warn, ...) được
// tách riêng thành nhóm "log" vì không gửi cho model; phần còn lại tính là "model".
//
// Đánh dấu bỏ qua (dành cho dữ liệu chỉ dùng khi language=zh):
//   - dòng chứa comment `//check-han:ok` bỏ qua literal trên dòng đó;
//   - file có comment `//check-han:ignore-file` bị bỏ qua toàn bộ.
//
// Dùng: go run ./scripts/check-han [-v] [-root .] [-skip-log]
// Mã thoát 1 khi còn chữ Hán (sau khi áp -skip-log), 0 khi sạch.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	markOK         = "check-han:ok"
	markIgnoreFile = "check-han:ignore-file"
)

// translatedDirs là thư mục tài nguyên tiếng Việt, tương đối với root.
var translatedDirs = []string{"assets/prompts/vi", "assets/references/vi", "assets/styles/vi"}

type hit struct {
	file string
	line int
	kind string // model | log | asset
	text string
}

func main() {
	root := flag.String("root", ".", "thư mục gốc của repo")
	verbose := flag.Bool("v", false, "in từng dòng còn chữ Hán")
	skipLog := flag.Bool("skip-log", false, "không tính literal log vào mã thoát")
	punct := flag.Bool("punct", false, "liệt kê thêm literal có dấu câu toàn chiều rộng kiểu CJK như （）：，。 (không tính vào mã thoát)")
	flag.Parse()
	checkPunct = *punct

	hits, err := scan(*root)
	if err == nil {
		var missing []hit
		missing, err = missingTranslations(*root)
		hits = append(hits, missing...)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "check-han:", err)
		os.Exit(2)
	}
	os.Exit(report(os.Stdout, hits, *verbose, *skipLog))
}

func scan(root string) ([]hit, error) {
	var hits []hit
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "testdata", "zh", "node_modules", "plans", "reports":
				return fs.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go"):
			h, err := scanGo(path, rel)
			if err != nil {
				return err
			}
			hits = append(hits, h...)
		case strings.HasSuffix(rel, ".md") && inTranslated(rel):
			h, err := scanText(path, rel)
			if err != nil {
				return err
			}
			hits = append(hits, h...)
		}
		return nil
	})
	return hits, err
}

// missingTranslations báo các tài nguyên gốc chưa có bản vi tương ứng
// (quy ước đường dẫn giống assets.localPath).
func missingTranslations(root string) ([]hit, error) {
	var hits []hit
	check := func(base, vi string) {
		if _, err := os.Stat(filepath.Join(root, vi)); err != nil {
			hits = append(hits, hit{file: vi, line: 0, kind: "asset", text: "thiếu bản dịch của " + base})
		}
	}
	check("assets/voice.md", "assets/prompts/vi/voice.md")
	for _, dir := range []string{"prompts", "references", "styles"} {
		base := filepath.Join(root, "assets", dir)
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "vi" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".md") {
				return nil
			}
			rel, _ := filepath.Rel(base, path)
			rel = filepath.ToSlash(rel)
			check("assets/"+dir+"/"+rel, "assets/"+dir+"/vi/"+rel)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return hits, nil
}

func inTranslated(rel string) bool {
	for _, d := range translatedDirs {
		if strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

// checkPunct bật báo cáo dấu câu CJK (nhóm "punct", chỉ để tham khảo).
var checkPunct bool

const cjkPunct = "（）：，。；！？、「」《》【】"

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func scanGo(path, rel string) ([]hit, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", rel, err)
	}
	okLines := map[int]bool{}
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, markIgnoreFile) {
				return nil, nil
			}
			if strings.Contains(c.Text, markOK) {
				okLines[fset.Position(c.Pos()).Line] = true
			}
		}
	}
	logArgs := map[*ast.BasicLit]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isLogCall(call) {
			return true
		}
		for _, a := range call.Args {
			if lit, ok := a.(*ast.BasicLit); ok {
				logArgs[lit] = true
			}
		}
		return true
	})
	var hits []hit
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || (lit.Kind != token.STRING && lit.Kind != token.CHAR) {
			return true
		}
		val := lit.Value
		if u, err := strconv.Unquote(val); err == nil {
			val = u
		}
		line := fset.Position(lit.Pos()).Line
		if okLines[line] {
			return true
		}
		kind := "model"
		switch {
		case hasHan(val):
			if logArgs[lit] {
				kind = "log"
			}
		case checkPunct && strings.ContainsAny(val, cjkPunct):
			kind = "punct"
		default:
			return true
		}
		hits = append(hits, hit{file: rel, line: line, kind: kind, text: oneLine(val)})
		return true
	})
	return hits, nil
}

// isLogCall nhận diện slog.X(...), logger.X(...), x.log.X(...) với X là mức log.
func isLogCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch strings.TrimSuffix(sel.Sel.Name, "Context") {
	case "Debug", "Info", "Warn", "Error", "Log":
	default:
		return false
	}
	var recv string
	switch x := sel.X.(type) {
	case *ast.Ident:
		recv = x.Name
	case *ast.SelectorExpr:
		recv = x.Sel.Name
	case *ast.CallExpr: // slog.With(...).Info / logger.L().Info
		return true
	}
	recv = strings.ToLower(recv)
	return recv == "slog" || strings.Contains(recv, "log")
}

func scanText(path, rel string) ([]hit, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var hits []hit
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for n := 1; sc.Scan(); n++ {
		if hasHan(sc.Text()) {
			hits = append(hits, hit{file: rel, line: n, kind: "asset", text: oneLine(sc.Text())})
		}
	}
	return hits, sc.Err()
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 80 {
		s = string(r[:80]) + "…"
	}
	return s
}

type fileStat struct {
	file                     string
	model, log, asset, punct int
}

func report(w *os.File, hits []hit, verbose, skipLog bool) int {
	stats := map[string]*fileStat{}
	var totalModel, totalLog, totalAsset, totalPunct int
	for _, h := range hits {
		st := stats[h.file]
		if st == nil {
			st = &fileStat{file: h.file}
			stats[h.file] = st
		}
		switch h.kind {
		case "model":
			st.model++
			totalModel++
		case "log":
			st.log++
			totalLog++
		case "punct":
			st.punct++
			totalPunct++
		default:
			st.asset++
			totalAsset++
		}
	}
	list := make([]*fileStat, 0, len(stats))
	for _, st := range stats {
		list = append(list, st)
	}
	sort.Slice(list, func(i, j int) bool {
		a, b := list[i].model+list[i].asset+list[i].log+list[i].punct, list[j].model+list[j].asset+list[j].log+list[j].punct
		if a != b {
			return a > b
		}
		return list[i].file < list[j].file
	})
	fmt.Fprintf(w, "%-6s %-6s %-6s %-6s  %s\n", "model", "log", "asset", "punct", "file")
	for _, st := range list {
		fmt.Fprintf(w, "%-6d %-6d %-6d %-6d  %s\n", st.model, st.log, st.asset, st.punct, st.file)
	}
	if verbose {
		fmt.Fprintln(w)
		for _, h := range hits {
			fmt.Fprintf(w, "%s:%d [%s] %s\n", h.file, h.line, h.kind, h.text)
		}
	}
	fmt.Fprintf(w, "\nTỔNG: model=%d log=%d asset=%d (gửi cho model = model+asset = %d)\n",
		totalModel, totalLog, totalAsset, totalModel+totalAsset)
	if checkPunct {
		fmt.Fprintf(w, "Dấu câu CJK (tham khảo, không tính mã thoát): punct=%d\n", totalPunct)
	}
	bad := totalModel + totalAsset
	if !skipLog {
		bad += totalLog
	}
	if bad > 0 {
		return 1
	}
	return 0
}
