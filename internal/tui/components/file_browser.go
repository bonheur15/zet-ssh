package components

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FileItem struct {
	Info os.FileInfo
}

func (i FileItem) Title() string {
	if i.Info.IsDir() {
		return "📁 " + i.Info.Name()
	}
	return "📄 " + i.Info.Name()
}

func (i FileItem) Description() string {
	size := i.Info.Size()
	return fmt.Sprintf("%d bytes | %s", size, i.Info.ModTime().Format("2006-01-02 15:04"))
}

func (i FileItem) FilterValue() string { return i.Info.Name() }

type FileBrowser struct {
	list  list.Model
	path  string
	width int
	height int
}

func NewFileBrowser(path string, width, height int) FileBrowser {
	items := getFiles(path)
	l := list.New(items, list.NewDefaultDelegate(), width, height)
	l.Title = "File Browser: " + path
	l.SetShowHelp(false)

	return FileBrowser{
		list:   l,
		path:   path,
		width:  width,
		height: height,
	}
}

func getFiles(path string) []list.Item {
	entries, err := os.ReadDir(path)
	if err != nil {
		return []list.Item{}
	}

	var items []list.Item
	// Add parent directory entry
	// items = append(items, FileItem{...}) 

	for _, e := range entries {
		info, _ := e.Info()
		items = append(items, FileItem{Info: info})
	}
	return items
}

func (m FileBrowser) Update(msg tea.Msg) (FileBrowser, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m FileBrowser) View() string {
	return m.list.View()
}
