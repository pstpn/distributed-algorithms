package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pstpn/iidx/internal/engine"
	"github.com/pstpn/iidx/internal/index"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7DC4E4")).
			MarginBottom(1)

	resultTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A6DA95")).
				Bold(true)

	scoreStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F5C2E7"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1E1E2E")).
			Background(lipgloss.Color("#F5C2E7")).
			Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CAD3F5"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5B6078"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8AADF4")).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ED8796"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8AADF4")).
			Bold(true).
			MarginBottom(1)

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F5C2E7")).
			Bold(true)

	focusIndicatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A6DA95")).
				Bold(true)

	checkboxStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6DA95"))

	checkboxDimStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5B6078"))

	configSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1E1E2E")).
				Background(lipgloss.Color("#8AADF4"))

	configNormalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CAD3F5"))

	modeSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1E1E2E")).
				Background(lipgloss.Color("#8AADF4"))

	modeNormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CAD3F5"))
)

type DocRecord struct {
	ID    uint32 `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type SearchResultItem struct {
	DocID uint32
	Score float64
	Title string
}

type fileInfo struct {
	path     string
	name     string
	size     int64
	docCount int
	selected bool
}

type indexFileInfo struct {
	path    string
	name    string
	size    int64
	modTime time.Time
}

type viewState int

const (
	viewModeSelect viewState = iota
	viewConfig
	viewIndexSelect
	viewLoading
	viewSearch
	viewDocument
)

type focusState int

const (
	focusInput focusState = iota
	focusResults
)

type filesScannedMsg struct {
	files []fileInfo
	err   error
}

type indexFilesScannedMsg struct {
	files []indexFileInfo
	err   error
}

type indexLoadedMsg struct {
	eng     *engine.Engine
	numDocs int
	err     error
}

type searchResultMsg struct {
	results []SearchResultItem
	err     error
}

type progressMsg struct {
	loaded int
	total  int
	phase  string
}

type model struct {
	state          viewState
	focus          focusState
	eng            *engine.Engine
	input          textinput.Model
	results        []SearchResultItem
	selected       int
	scroll         int
	err            error
	docView        []string
	docTitle       string
	docScroll      int
	termWidth      int
	termHeight     int
	dataDir        string
	indexDir       string
	availableFiles []fileInfo
	indexFiles     []indexFileInfo
	configCursor   int
	modeCursor     int
	indexCursor    int
	loadStatus     string
	progressCh     chan progressMsg
	loadProgress   int
	loadTotal      int
	loadPhase      string
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteByte(' ')
		}
		result.WriteRune(c)
	}
	return result.String()
}

func formatDate(t time.Time) string {
	return t.Format("02.01.2006 15:04")
}

func scanFilesCmd(dataDir string) tea.Cmd {
	return func() tea.Msg {
		files, err := filepath.Glob(filepath.Join(dataDir, "*.jsonl"))
		if err != nil {
			return filesScannedMsg{err: fmt.Errorf("glob: %w", err)}
		}
		if len(files) == 0 {
			return filesScannedMsg{err: fmt.Errorf("no .jsonl files in %s", dataDir)}
		}

		var infos []fileInfo
		for _, f := range files {
			stat, err := os.Stat(f)
			if err != nil {
				continue
			}
			docCount := countLines(f)
			infos = append(infos, fileInfo{
				path:     f,
				name:     filepath.Base(f),
				size:     stat.Size(),
				docCount: docCount,
				selected: false,
			})
		}

		if len(infos) == 0 {
			return filesScannedMsg{err: fmt.Errorf("no readable .jsonl files in %s", dataDir)}
		}

		half := len(infos) / 2
		if half == 0 {
			half = 1
		}
		for i := 0; i < half; i++ {
			infos[i].selected = true
		}

		return filesScannedMsg{files: infos}
	}
}

func scanIndexFilesCmd(indexDir string) tea.Cmd {
	return func() tea.Msg {
		files, err := filepath.Glob(filepath.Join(indexDir, "*.idx"))
		if err != nil {
			return indexFilesScannedMsg{err: fmt.Errorf("glob: %w", err)}
		}
		if len(files) == 0 {
			return indexFilesScannedMsg{err: fmt.Errorf("no .idx files in %s", indexDir)}
		}

		var infos []indexFileInfo
		for _, f := range files {
			stat, err := os.Stat(f)
			if err != nil {
				continue
			}
			infos = append(infos, indexFileInfo{
				path:    f,
				name:    filepath.Base(f),
				size:    stat.Size(),
				modTime: stat.ModTime(),
			})
		}

		if len(infos) == 0 {
			return indexFilesScannedMsg{err: fmt.Errorf("no readable .idx files in %s", indexDir)}
		}

		sort.Slice(infos, func(i, j int) bool {
			return infos[i].modTime.After(infos[j].modTime)
		})

		return indexFilesScannedMsg{files: infos}
	}
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0), 10*1024*1024)
	for scanner.Scan() {
		count++
	}
	return count
}

func loadIndexCmd(selectedFiles []fileInfo, indexDir string, ch chan progressMsg) tea.Cmd {
	return func() tea.Msg {
		b := index.NewIndexBuilder()
		totalDocs := 0

		totalExpected := 0
		for _, fi := range selectedFiles {
			totalExpected += fi.docCount
		}

		for _, fi := range selectedFiles {
			file, err := os.Open(fi.path)
			if err != nil {
				continue
			}

			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 0), 10*1024*1024)

			count := 0
			for scanner.Scan() {
				var doc DocRecord
				if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
					continue
				}
				b.AddDocument(doc.Title, doc.Text)
				count++
				totalDocs++

				if totalDocs%200 == 0 {
					ch <- progressMsg{
						loaded: totalDocs,
						total:  totalExpected,
						phase:  fmt.Sprintf("Чтение: %s", fi.name),
					}
				}
			}
			file.Close()

			ch <- progressMsg{
				loaded: totalDocs,
				total:  totalExpected,
				phase:  fmt.Sprintf("Чтение: %s", fi.name),
			}
		}

		ch <- progressMsg{
			loaded: totalDocs,
			total:  totalExpected,
			phase:  "Сохранение индекса на диск",
		}

		if err := os.MkdirAll(indexDir, 0755); err != nil {
			return indexLoadedMsg{err: fmt.Errorf("create index dir: %w", err)}
		}

		idxPath := filepath.Join(indexDir, "index-"+time.Now().Format("20060102-150405")+".idx")

		if err := b.Save(idxPath); err != nil {
			os.Remove(idxPath)
			os.Remove(index.DocStoreFilename(idxPath))
			return indexLoadedMsg{err: fmt.Errorf("save index: %w", err)}
		}

		ch <- progressMsg{
			loaded: totalDocs,
			total:  totalExpected,
			phase:  "Загрузка индекса в память (mmap)",
		}

		eng, err := engine.LoadEngine(idxPath)
		if err != nil {
			os.Remove(idxPath)
			os.Remove(index.DocStoreFilename(idxPath))
			return indexLoadedMsg{err: fmt.Errorf("load index: %w", err)}
		}

		close(ch)
		return indexLoadedMsg{eng: eng, numDocs: totalDocs}
	}
}

func waitForProgressCmd(ch chan progressMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func loadExistingIndexCmd(idxPath string) tea.Cmd {
	return func() tea.Msg {
		eng, err := engine.LoadEngine(idxPath)
		if err != nil {
			return indexLoadedMsg{err: fmt.Errorf("load index: %w", err)}
		}
		return indexLoadedMsg{eng: eng, numDocs: int(eng.NumDocs())}
	}
}

func searchCmd(eng *engine.Engine, query string) tea.Cmd {
	return func() tea.Msg {
		result, err := eng.Search(query)
		if err != nil {
			return searchResultMsg{err: err}
		}

		items := make([]SearchResultItem, 0, len(result.Docs))
		for _, sd := range result.Docs {
			title := eng.GetDocumentTitle(sd.DocID)
			if title == "" {
				title = fmt.Sprintf("Document #%d", sd.DocID)
			}
			items = append(items, SearchResultItem{
				DocID: sd.DocID,
				Score: sd.Score,
				Title: title,
			})
		}

		return searchResultMsg{results: items}
	}
}

func initialModel(dataDir, indexDir string) model {
	ti := textinput.New()
	ti.Placeholder = "cat AND dog"
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60

	return model{
		state:    viewModeSelect,
		focus:    focusInput,
		input:    ti,
		dataDir:  dataDir,
		indexDir: indexDir,
	}
}

func (m model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 80
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		var currentLine strings.Builder
		for _, word := range words {
			if currentLine.Len() == 0 {
				currentLine.WriteString(word)
			} else if currentLine.Len()+1+len(word) <= width {
				currentLine.WriteByte(' ')
				currentLine.WriteString(word)
			} else {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
				currentLine.WriteString(word)
			}
		}
		if currentLine.Len() > 0 {
			lines = append(lines, currentLine.String())
		}
	}
	return lines
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		if m.termWidth > 4 {
			m.input.Width = m.termWidth - 4
		}
		if m.state == viewDocument {
			m.rewrapDocument()
		}
		return m, nil

	case filesScannedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.availableFiles = msg.files
		return m, nil

	case indexFilesScannedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.indexFiles = msg.files
		return m, nil

	case progressMsg:
		m.loadProgress = msg.loaded
		m.loadTotal = msg.total
		m.loadPhase = msg.phase
		if m.progressCh != nil {
			return m, waitForProgressCmd(m.progressCh)
		}
		return m, nil

	case indexLoadedMsg:
		if msg.err != nil {
			m.state = viewSearch
			m.err = msg.err
			m.progressCh = nil
			return m, nil
		}
		m.eng = msg.eng
		m.loadStatus = fmt.Sprintf("Загружено документов: %d", msg.numDocs)
		m.state = viewSearch
		m.progressCh = nil
		return m, nil

	case searchResultMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.results = msg.results
		m.selected = 0
		m.scroll = 0
		m.err = nil
		if len(m.results) > 0 {
			m.focus = focusResults
			m.input.Blur()
		}
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch m.state {
		case viewModeSelect:
			return m.handleModeSelectKey(keyMsg)
		case viewConfig:
			return m.handleConfigKey(keyMsg)
		case viewIndexSelect:
			return m.handleIndexSelectKey(keyMsg)
		case viewLoading:
			if keyMsg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		case viewSearch:
			return m.handleSearchKey(keyMsg)
		case viewDocument:
			return m.handleDocumentKey(keyMsg)
		}
		return m, nil
	}

	if m.state == viewSearch && m.focus == focusInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) handleModeSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		if m.modeCursor > 0 {
			m.modeCursor--
		}

	case "down", "j":
		if m.modeCursor < 1 {
			m.modeCursor++
		}

	case "enter":
		m.err = nil
		switch m.modeCursor {
		case 0:
			m.state = viewConfig
			m.configCursor = 0
			m.availableFiles = nil
			return m, scanFilesCmd(m.dataDir)
		case 1:
			m.state = viewIndexSelect
			m.indexCursor = 0
			m.indexFiles = nil
			return m, scanIndexFilesCmd(m.indexDir)
		}
	}

	return m, nil
}

func (m model) handleConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.state = viewModeSelect
		m.err = nil
		m.availableFiles = nil
		m.configCursor = 0
		return m, nil

	case "up", "k":
		if m.configCursor > 0 {
			m.configCursor--
		}

	case "down", "j":
		if m.configCursor < len(m.availableFiles)-1 {
			m.configCursor++
		}

	case " ":
		if m.configCursor < len(m.availableFiles) {
			m.availableFiles[m.configCursor].selected = !m.availableFiles[m.configCursor].selected
		}

	case "a":
		for i := range m.availableFiles {
			m.availableFiles[i].selected = true
		}

	case "n":
		for i := range m.availableFiles {
			m.availableFiles[i].selected = false
		}

	case "enter":
		var selected []fileInfo
		for _, f := range m.availableFiles {
			if f.selected {
				selected = append(selected, f)
			}
		}
		if len(selected) == 0 {
			m.err = fmt.Errorf("выберите хотя бы один файл")
			return m, nil
		}
		m.err = nil
		m.state = viewLoading
		m.loadProgress = 0
		m.loadTotal = 0
		m.loadPhase = ""
		ch := make(chan progressMsg, 64)
		m.progressCh = ch
		return m, tea.Batch(loadIndexCmd(selected, m.indexDir, ch), waitForProgressCmd(ch))
	}

	return m, nil
}

func (m model) handleIndexSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.state = viewModeSelect
		m.err = nil
		m.indexFiles = nil
		m.indexCursor = 0
		return m, nil

	case "up", "k":
		if m.indexCursor > 0 {
			m.indexCursor--
		}

	case "down", "j":
		if m.indexCursor < len(m.indexFiles)-1 {
			m.indexCursor++
		}

	case "enter":
		if len(m.indexFiles) == 0 || m.indexCursor >= len(m.indexFiles) {
			return m, nil
		}
		m.err = nil
		m.state = viewLoading
		return m, loadExistingIndexCmd(m.indexFiles[m.indexCursor].path)
	}

	return m, nil
}

func (m model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if msg.String() == "tab" {
		if m.focus == focusInput && len(m.results) > 0 {
			m.focus = focusResults
			m.input.Blur()
		} else if m.focus == focusResults {
			m.focus = focusInput
			m.input.Focus()
		}
		return m, nil
	}

	if m.focus == focusInput {
		return m.handleInputFocus(msg)
	}
	return m.handleResultsFocus(msg)
}

func (m model) handleInputFocus(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		query := m.input.Value()
		if query == "" {
			m.results = nil
			return m, nil
		}
		m.err = nil
		return m, searchCmd(m.eng, query)

	case "down":
		if len(m.results) > 0 {
			m.focus = focusResults
			m.input.Blur()
		}

	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) handleResultsFocus(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selected > 0 {
			m.selected--
			if m.selected < m.scroll {
				m.scroll = m.selected
			}
		} else {
			m.focus = focusInput
			m.input.Focus()
		}

	case "down", "j":
		if m.selected < len(m.results)-1 {
			m.selected++
			visibleHeight := m.termHeight - 8
			if visibleHeight < 3 {
				visibleHeight = 3
			}
			if m.selected >= m.scroll+visibleHeight {
				m.scroll = m.selected - visibleHeight + 1
			}
		}

	case "enter", "o":
		if len(m.results) > 0 && m.selected < len(m.results) {
			r := m.results[m.selected]
			m.docTitle = r.Title
			m.rewrapDocument()
			m.docScroll = 0
			m.state = viewDocument
		}
	}

	return m, nil
}

func (m model) handleDocumentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "backspace":
		m.state = viewSearch
		m.docScroll = 0
		return m, nil

	case "up", "k":
		if m.docScroll > 0 {
			m.docScroll--
		}

	case "down", "j":
		if m.docScroll < len(m.docView)-1 {
			m.docScroll++
		}

	case "pgup":
		m.docScroll -= 20
		if m.docScroll < 0 {
			m.docScroll = 0
		}

	case "pgdown":
		m.docScroll += 20
		if m.docScroll >= len(m.docView) {
			m.docScroll = len(m.docView) - 1
		}

	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) rewrapDocument() {
	if len(m.results) == 0 || m.selected >= len(m.results) {
		return
	}
	text := m.eng.GetDocument(m.results[m.selected].DocID)
	width := m.termWidth - 2
	if width <= 0 {
		width = 80
	}
	m.docView = wrapText(text, width)
}

func (m model) View() string {
	switch m.state {
	case viewModeSelect:
		return m.viewModeSelect()
	case viewConfig:
		return m.viewConfig()
	case viewIndexSelect:
		return m.viewIndexSelect()
	case viewLoading:
		return m.viewLoading()
	case viewSearch:
		return m.viewSearch()
	case viewDocument:
		return m.viewDocument()
	}
	return ""
}

func (m model) viewModeSelect() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Inverted Index"))
	b.WriteString("\n\n")

	b.WriteString(headerStyle.Render("Выберите режим работы:"))
	b.WriteString("\n")

	modes := []struct {
		text string
		desc string
	}{
		{"Построить индекс и загрузить", fmt.Sprintf("(источник: %s → индекс: %s)", m.dataDir, m.indexDir)},
		{"Загрузить существующий индекс", fmt.Sprintf("(папка: %s)", m.indexDir)},
	}

	for i, mode := range modes {
		line := " " + mode.text
		if i == m.modeCursor {
			b.WriteString(modeSelectedStyle.Render(" ▸" + line))
		} else {
			b.WriteString(modeNormalStyle.Render("  " + line))
		}
		b.WriteString("\n")

		desc := fmt.Sprintf("      %s", mode.desc)
		if i == m.modeCursor {
			b.WriteString(dimStyle.Render(desc))
		} else {
			b.WriteString(dimStyle.Render(desc))
		}
		b.WriteString("\n\n")
	}

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Ошибка: %v", m.err)))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  ↑/↓: навигация │ Enter: выбрать │ Ctrl+C: выход"))

	return b.String()
}

func (m model) viewConfig() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Настройка индекса"))
	b.WriteString("\n\n")

	if len(m.availableFiles) == 0 {
		if m.err != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("  Ошибка: %v", m.err)))
			b.WriteString("\n\n")
		}
		b.WriteString(loadingStyle.Render("  Сканирование файлов..."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("  Esc: назад │ Ctrl+C: выход"))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Ошибка: %v", m.err)))
		b.WriteString("\n")
	}

	b.WriteString(headerStyle.Render("  Файлы документов:"))
	b.WriteString("\n")

	maxNameLen := 0
	for _, f := range m.availableFiles {
		if len(f.name) > maxNameLen {
			maxNameLen = len(f.name)
		}
	}
	if maxNameLen > 40 {
		maxNameLen = 40
	}

	for i, f := range m.availableFiles {
		checkbox := "[ ]"
		style := checkboxDimStyle
		if f.selected {
			checkbox = "[✓]"
			style = checkboxStyle
		}

		name := f.name
		if len(name) > maxNameLen {
			name = name[:maxNameLen-3] + "..."
		}
		name = name + strings.Repeat(" ", maxNameLen-len(name))

		line := fmt.Sprintf("  %s %s  %s  %s док.",
			style.Render(checkbox),
			name,
			dimStyle.Render(formatSize(f.size)),
			dimStyle.Render(formatInt(f.docCount)),
		)

		if i == m.configCursor {
			b.WriteString(configSelectedStyle.Render(" ▸" + line))
		} else {
			b.WriteString(configNormalStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	selectedCount := 0
	totalDocs := 0
	var totalSize int64
	for _, f := range m.availableFiles {
		if f.selected {
			selectedCount++
			totalDocs += f.docCount
			totalSize += f.size
		}
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Выбрано: %d/%d файлов │ ~%s док. │ %s",
		selectedCount, len(m.availableFiles), formatInt(totalDocs), formatSize(totalSize))))
	b.WriteString("\n")

	b.WriteString(helpStyle.Render("  ↑/↓: навигация │ Space: выбрать │ A: все │ N: снять │ Enter: загрузить │ Esc: назад │ Ctrl+C: выход"))

	return b.String()
}

func (m model) viewIndexSelect() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Загрузка индекса"))
	b.WriteString("\n\n")

	if len(m.indexFiles) == 0 {
		if m.err != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("  Ошибка: %v", m.err)))
			b.WriteString("\n\n")
		} else {
			b.WriteString(loadingStyle.Render("  Сканирование файлов индексов..."))
			b.WriteString("\n\n")
		}
		b.WriteString(helpStyle.Render("  Esc: назад │ Ctrl+C: выход"))
		return b.String()
	}

	b.WriteString(headerStyle.Render(fmt.Sprintf("  Доступные индексы (%s):", m.indexDir)))
	b.WriteString("\n")

	maxNameLen := 0
	for _, f := range m.indexFiles {
		if len(f.name) > maxNameLen {
			maxNameLen = len(f.name)
		}
	}
	if maxNameLen > 50 {
		maxNameLen = 50
	}

	for i, f := range m.indexFiles {
		name := f.name
		if len(name) > maxNameLen {
			name = name[:maxNameLen-3] + "..."
		}
		name = name + strings.Repeat(" ", maxNameLen-len(name))

		line := fmt.Sprintf("  %s  %s  %s", name, dimStyle.Render(formatSize(f.size)), dimStyle.Render(formatDate(f.modTime)))

		if i == m.indexCursor {
			b.WriteString(configSelectedStyle.Render(" ▸" + line))
		} else {
			b.WriteString(configNormalStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  ↑/↓: навигация │ Enter: загрузить │ Esc: назад │ Ctrl+C: выход"))

	return b.String()
}

func (m model) viewLoading() string {
	var b strings.Builder

	b.WriteString("\n\n   ")
	b.WriteString(loadingStyle.Render("Построение индекса..."))
	b.WriteString("\n\n")

	barWidth := 40
	if m.termWidth > 20 {
		barWidth = min(m.termWidth-20, 50)
	}

	if m.loadTotal > 0 {
		pct := float64(m.loadProgress) / float64(m.loadTotal)
		filled := int(pct * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}

		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		pctStr := fmt.Sprintf("%5.1f%%", pct*100)

		b.WriteString("   ")
		b.WriteString(scoreStyle.Render(bar))
		b.WriteString(" ")
		b.WriteString(dimStyle.Render(pctStr))
		b.WriteString("\n")

		b.WriteString("   ")
		b.WriteString(dimStyle.Render(fmt.Sprintf("%s / %s документов", formatInt(m.loadProgress), formatInt(m.loadTotal))))
		b.WriteString("\n")
	} else {
		bar := strings.Repeat("░", barWidth)
		b.WriteString("   ")
		b.WriteString(dimStyle.Render(bar))
		b.WriteString("\n")
	}

	if m.loadPhase != "" {
		b.WriteString("\n   ")
		b.WriteString(dimStyle.Render(m.loadPhase))
		b.WriteString("\n")
	}

	b.WriteString("\n   ")
	b.WriteString(dimStyle.Render("Ctrl+C: отмена"))

	return b.String()
}

func (m model) viewSearch() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Inverted Index"))
	b.WriteString("\n")

	if m.eng != nil {
		statsLine := fmt.Sprintf("Документов: %d │ Операторы: AND OR NOT ADJ NEAR/n", m.eng.NumDocs())
		if m.loadStatus != "" {
			statsLine = m.loadStatus + " │ Операторы: AND OR NOT ADJ NEAR/n"
		}
		b.WriteString(dimStyle.Render(statsLine))
		b.WriteString("\n")
	}

	if m.focus == focusInput {
		b.WriteString(focusIndicatorStyle.Render("▸ "))
	} else {
		b.WriteString("  ")
	}
	b.WriteString(m.input.View())
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Ошибка: %v", m.err)))
		b.WriteString("\n")
	}

	if len(m.results) > 0 {
		focusLabel := ""
		if m.focus == focusResults {
			focusLabel = focusIndicatorStyle.Render(" ◂ результаты")
		}
		b.WriteString(headerStyle.Render(fmt.Sprintf("Найдено: %d документов", len(m.results))) + focusLabel)
		b.WriteString("\n")

		visibleHeight := m.termHeight - 8
		if visibleHeight < 3 {
			visibleHeight = 3
		}

		end := m.scroll + visibleHeight
		if end > len(m.results) {
			end = len(m.results)
		}

		maxScore := 0.0
		for _, r := range m.results {
			if r.Score > maxScore {
				maxScore = r.Score
			}
		}

		for i := m.scroll; i < end; i++ {
			r := m.results[i]
			numStr := fmt.Sprintf("%3d.", i+1)
			scoreStr := fmt.Sprintf("%8.4f", r.Score)

			barLen := 0
			if maxScore > 0 {
				barLen = int(r.Score / maxScore * 10)
			}
			bar := strings.Repeat("█", barLen) + strings.Repeat("░", 10-barLen)

			if i == m.selected && m.focus == focusResults {
				line := fmt.Sprintf(" ▸ %s %s %s %s", numStr, scoreStyle.Render(scoreStr), dimStyle.Render(bar), r.Title)
				b.WriteString(selectedStyle.Render(line))
			} else if i == m.selected {
				line := fmt.Sprintf(" ▸ %s %s %s %s", numStr, scoreStyle.Render(scoreStr), dimStyle.Render(bar), resultTitleStyle.Render(r.Title))
				b.WriteString(normalItemStyle.Render(line))
			} else {
				line := fmt.Sprintf("   %s %s %s %s", numStr, scoreStyle.Render(scoreStr), dimStyle.Render(bar), resultTitleStyle.Render(r.Title))
				b.WriteString(normalItemStyle.Render(line))
			}
			b.WriteString("\n")
		}

		if end < len(m.results) {
			b.WriteString(dimStyle.Render(fmt.Sprintf("   ... и ещё %d результатов", len(m.results)-end)))
			b.WriteString("\n")
		}
	} else if m.input.Value() != "" && m.err == nil && m.eng != nil {
		b.WriteString(dimStyle.Render("Нажмите Enter для поиска"))
		b.WriteString("\n")
	}

	var help string
	if m.focus == focusInput {
		help = helpStyle.Render("Enter: поиск │ Tab: к результатам │ Ctrl+C: выход")
	} else {
		help = helpStyle.Render("↑/k ↓/j: навигация │ Enter: открыть │ Tab: к вводу │ Ctrl+C: выход")
	}
	b.WriteString(help)

	return b.String()
}

func (m model) viewDocument() string {
	var b strings.Builder

	var docHeader string
	if m.selected < len(m.results) {
		r := m.results[m.selected]
		docHeader = fmt.Sprintf("📄 %s  %s", r.Title, scoreStyle.Render(fmt.Sprintf("[релевантность: %.4f]", r.Score)))
	} else {
		docHeader = fmt.Sprintf("📄 %s", m.docTitle)
	}
	b.WriteString(headerStyle.Render(docHeader))
	b.WriteString("\n")

	totalLines := len(m.docView)
	visibleHeight := m.termHeight - 4
	if visibleHeight < 3 {
		visibleHeight = 3
	}

	startLine := m.docScroll
	endLine := startLine + visibleHeight
	if endLine > totalLines {
		endLine = totalLines
	}
	if startLine > totalLines {
		startLine = totalLines
	}

	for i := startLine; i < endLine; i++ {
		b.WriteString(m.docView[i])
		b.WriteString("\n")
	}

	if totalLines > 0 {
		pct := float64(m.docScroll+1) / float64(totalLines) * 100
		if pct > 100 {
			pct = 100
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("Строка %d/%d (%.0f%%)", m.docScroll+1, totalLines, pct)))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("Esc/q: назад │ ↑/k ↓/j: скролл │ PgUp/PgDn: быстро │ Ctrl+C: выход"))

	return b.String()
}

func main() {
	dataDir := flag.String("data", "data/documents", "директория с JSONL файлами документов")
	indexDir := flag.String("index-dir", "data/indexes", "директория с файлами индексов")
	flag.Parse()

	m := initialModel(*dataDir, *indexDir)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatalf("TUI error: %v", err)
	}
}
