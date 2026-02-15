package components

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"zet-ssh/internal/tui/theme"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type FileSource interface {
	ListDir(path string) ([]os.FileInfo, error)
	PathSeparator() string
}

type LocalSource struct{}

func (s LocalSource) ListDir(path string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var infos []os.FileInfo
	for _, e := range entries {
		info, _ := e.Info()
		infos = append(infos, info)
	}
	return infos, nil
}

func (s LocalSource) PathSeparator() string { return string(os.PathSeparator) }

type FileItem struct {
	Info     os.FileInfo
	Name     string
	IsParent bool
}

func (i FileItem) Title() string {
	if i.IsParent {
		return "↩ .."
	}
	name := i.Info.Name()
	if i.Name != "" {
		name = i.Name
	}
	if i.Info.IsDir() {
		return "📁 " + name
	}
	return "📄 " + name
}

func (i FileItem) Description() string {
	if i.IsParent {
		return "Parent directory"
	}
	size := i.Info.Size()
	return fmt.Sprintf("%d bytes | %s", size, i.Info.ModTime().Format("15:04"))
}

func (i FileItem) FilterValue() string {
	if i.IsParent {
		return ".."
	}
	return i.Info.Name()
}

type FileBrowser struct {
	list   list.Model
	path   string
	source FileSource
	width  int
	height int
	Active bool
	errMsg string
}

func NewFileBrowser(source FileSource, path string, width, height int) FileBrowser {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), width, height)
	l.SetShowHelp(false)
	l.Title = path

	fb := FileBrowser{
		list:   l,
		path:   path,
		source: source,
		width:  width,
		height: height,
	}
	fb.Refresh()
	return fb
}

func (m *FileBrowser) Refresh() {
	files, err := m.source.ListDir(m.path)
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir() != files[j].IsDir() {
			return files[i].IsDir()
		}
		return strings.ToLower(files[i].Name()) < strings.ToLower(files[j].Name())
	})

	var items []list.Item
	if m.canGoUp() {
		items = append(items, FileItem{
			Name:     "..",
			IsParent: true,
		})
	}
	for _, f := range files {
		items = append(items, FileItem{Info: f})
	}
	m.list.SetItems(items)
	m.list.Title = m.path
	m.errMsg = ""
}

func (m FileBrowser) Update(msg tea.Msg) (FileBrowser, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m FileBrowser) View() string {
	style := theme.Border
	if m.Active {
		style = style.Copy().BorderForeground(theme.Accent)
	}
	body := m.list.View()
	if m.errMsg != "" {
		body = body + "\n" + theme.StatusError.Render(m.errMsg)
	}
	return style.Width(m.width).Height(m.height).Render(body)
}

func (m *FileBrowser) SetSize(w, h int) {
	m.width = w
	m.height = h
	listW := w - 2
	listH := h - 2
	if listW < 0 {
		listW = 0
	}
	if listH < 0 {
		listH = 0
	}
	m.list.SetSize(listW, listH)
}

func (m FileBrowser) CurrentPath() string {
	return m.path
}

func (m FileBrowser) SelectedItem() (FileItem, bool) {
	selected, ok := m.list.SelectedItem().(FileItem)
	return selected, ok
}

func (m FileBrowser) SelectedPath() (string, bool) {
	item, ok := m.SelectedItem()
	if !ok {
		return "", false
	}
	if item.IsParent {
		return m.parentPath(), true
	}
	return m.joinPath(item.Info.Name()), true
}

func (m *FileBrowser) EnterSelected() error {
	item, ok := m.SelectedItem()
	if !ok {
		return nil
	}
	if item.IsParent {
		return m.UpDir()
	}
	if !item.Info.IsDir() {
		return nil
	}
	m.path = m.joinPath(item.Info.Name())
	m.Refresh()
	return nil
}

func (m *FileBrowser) UpDir() error {
	if !m.canGoUp() {
		return nil
	}
	m.path = m.parentPath()
	m.Refresh()
	return nil
}

func (m *FileBrowser) SetPath(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	m.path = path
	m.Refresh()
}

func (m FileBrowser) joinPath(name string) string {
	if m.source.PathSeparator() == "/" {
		return path.Clean(path.Join(m.path, name))
	}
	return filepath.Clean(filepath.Join(m.path, name))
}

func (m FileBrowser) parentPath() string {
	if m.source.PathSeparator() == "/" {
		return path.Dir(m.path)
	}
	return filepath.Dir(m.path)
}

func (m FileBrowser) canGoUp() bool {
	parent := m.parentPath()
	return parent != m.path
}
