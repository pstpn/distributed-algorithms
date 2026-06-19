package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Document struct {
	ID    uint32 `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type Page struct {
	Title string `xml:"title"`
	NS    int    `xml:"ns"`
	ID    uint32 `xml:"id"`
	Text  string `xml:"revision>text"`
}

var (
	reHTMLComment  = regexp.MustCompile(`<!--[\s\S]*?-->`)
	reRef          = regexp.MustCompile(`<ref[^>]*>[\s\S]*?</ref>`)
	reRefSelfClose = regexp.MustCompile(`<ref[^/]*/>`)
	reGallery      = regexp.MustCompile(`<gallery>[\s\S]*?</gallery>`)
	reHTMLTag      = regexp.MustCompile(`<[^>]+>`)
	reCategoryLink = regexp.MustCompile(`\[\[Category:([^\]]+)\]\]`)
	rePipeLink     = regexp.MustCompile(`\[\[[^|\]]+\|([^\]]+)\]\]`)
	reWikiLink     = regexp.MustCompile(`\[\[([^|\]]+\|)?([^\]]+)\]\]`)
	reExtLink      = regexp.MustCompile(`\[https?://[^\s\]]+\s([^\]]+)\]`)
	reHeading      = regexp.MustCompile(`={2,}\s*([^=]+)\s*={2,}`)
	reBoldItalic   = regexp.MustCompile(`'{2,}([^']+)'{2,}`)
	reCurly2       = regexp.MustCompile(`\{\{[\s\S]*?\}\}`)
	reCurly1       = regexp.MustCompile(`\{[^\{]*\}`)
	reListMarkup   = regexp.MustCompile(`^[*#;:]+\s*`)
	reWhitespace   = regexp.MustCompile(`\s+`)
)

func cleanWikiText(text string) string {
	text = reHTMLComment.ReplaceAllString(text, "")
	text = reRef.ReplaceAllString(text, "")
	text = reRefSelfClose.ReplaceAllString(text, "")
	text = reGallery.ReplaceAllString(text, "")
	text = reHTMLTag.ReplaceAllString(text, "")

	for i := 0; i < 5; i++ {
		prev := text
		text = reCurly2.ReplaceAllString(text, "")
		if prev == text {
			break
		}
	}
	text = reCurly1.ReplaceAllString(text, "")

	text = reCategoryLink.ReplaceAllString(text, "$1")
	text = rePipeLink.ReplaceAllString(text, "$1")
	text = reWikiLink.ReplaceAllString(text, "$2")
	text = reExtLink.ReplaceAllString(text, "$1")
	text = reHeading.ReplaceAllString(text, "$1")
	text = reBoldItalic.ReplaceAllString(text, "$1")
	text = reListMarkup.ReplaceAllString(text, "")
	text = reWhitespace.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

func isArticle(page *Page) bool {
	if page.NS != 0 {
		return false
	}
	if page.Text == "" {
		return false
	}
	if strings.HasPrefix(page.Text, "#REDIRECT") || strings.HasPrefix(page.Text, "#redirect") {
		return false
	}
	return true
}

func openBZ2(path string) (io.ReadCloser, error) {
	if _, err := exec.LookPath("bzcat"); err == nil {
		cmd := exec.Command("bzcat", path)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("bzcat stdout pipe: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("bzcat start: %w", err)
		}
		return &cmdReadCloser{cmd: cmd, reader: stdout}, nil
	}

	if _, err := exec.LookPath("bzip2"); err == nil {
		cmd := exec.Command("bzip2", "-dc", path)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("bzip2 stdout pipe: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("bzip2 start: %w", err)
		}
		return &cmdReadCloser{cmd: cmd, reader: stdout}, nil
	}

	return nil, fmt.Errorf("не найден bzcat или bzip2 в PATH")
}

type cmdReadCloser struct {
	cmd    *exec.Cmd
	reader io.Reader
	closed bool
}

func (c *cmdReadCloser) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *cmdReadCloser) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	if closer, ok := c.reader.(io.Closer); ok {
		closer.Close()
	}
	return c.cmd.Wait()
}

func main() {
	inputDir := flag.String("input", "data/raw", "директория с .bz2 архивами Wikipedia XML")
	outputDir := flag.String("output", "data/documents", "директория для JSONL файлов")
	maxDocs := flag.Int("max", 0, "максимум документов на файл (0 = без лимита)")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		log.Fatalf("создание выходной директории: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(*inputDir, "*.bz2"))
	if err != nil {
		log.Fatalf("поиск файлов: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("не найдено .bz2 файлов в %s", *inputDir)
	}

	totalDocs := 0

	for _, bz2File := range files {
		baseName := strings.TrimSuffix(filepath.Base(bz2File), ".bz2")
		outputFile := filepath.Join(*outputDir, baseName+".jsonl")

		if _, err := os.Stat(outputFile); err == nil {
			log.Printf("Пропуск (уже существует): %s", filepath.Base(outputFile))
			continue
		}

		log.Printf("Обработка: %s → %s", filepath.Base(bz2File), filepath.Base(outputFile))

		count, err := processArchive(bz2File, outputFile, *maxDocs)
		if err != nil {
			log.Printf("ОШИБКА обработки %s: %v", bz2File, err)
			continue
		}

		totalDocs += count
		log.Printf("  → %d документов", count)
	}

	log.Printf("Готово. Всего документов: %d", totalDocs)
}

func processArchive(bz2File, outputFile string, maxDocs int) (int, error) {
	reader, err := openBZ2(bz2File)
	if err != nil {
		return 0, fmt.Errorf("открытие %s: %w", bz2File, err)
	}
	defer reader.Close()

	out, err := os.Create(outputFile)
	if err != nil {
		return 0, fmt.Errorf("создание %s: %w", outputFile, err)
	}
	defer out.Close()

	bufOut := bufio.NewWriterSize(out, 256*1024)
	defer bufOut.Flush()

	decoder := xml.NewDecoder(reader)
	docCount := 0

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return docCount, fmt.Errorf("чтение XML: %w", err)
		}

		se, ok := token.(xml.StartElement)
		if !ok || se.Name.Local != "page" {
			continue
		}

		var page Page
		if err := decoder.DecodeElement(&page, &se); err != nil {
			log.Printf("  пропуск страницы: %v", err)
			continue
		}

		if !isArticle(&page) {
			continue
		}

		cleanText := cleanWikiText(page.Text)
		if cleanText == "" {
			continue
		}

		doc := Document{
			ID:    page.ID,
			Title: page.Title,
			Text:  cleanText,
		}

		data, err := json.Marshal(doc)
		if err != nil {
			log.Printf("  кодировка JSON для id=%d: %v", page.ID, err)
			continue
		}

		if _, err := bufOut.Write(data); err != nil {
			return docCount, fmt.Errorf("запись в %s: %w", outputFile, err)
		}
		if _, err := bufOut.WriteString("\n"); err != nil {
			return docCount, fmt.Errorf("запись newline: %w", err)
		}

		docCount++

		if maxDocs > 0 && docCount >= maxDocs {
			break
		}

		if docCount%10000 == 0 {
			log.Printf("  обработано %d документов...", docCount)
		}
	}

	return docCount, nil
}
