package sftp

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Client struct {
	sftpClient *sftp.Client
}

type ProgressFunc func(copied int64, total int64)

func NewClient(sshClient *ssh.Client) (*Client, error) {
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	return &Client{sftpClient: sftpClient}, nil
}

func (c *Client) ListDir(path string) ([]os.FileInfo, error) {
	return c.sftpClient.ReadDir(path)
}

func (c *Client) Upload(localPath, remotePath string) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	if err := c.sftpClient.MkdirAll(path.Dir(remotePath)); err != nil {
		return err
	}

	remoteFile, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	return err
}

func (c *Client) UploadWithProgress(localPath, remotePath string, onProgress ProgressFunc, cancel <-chan struct{}) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	localInfo, err := localFile.Stat()
	if err != nil {
		return err
	}
	total := localInfo.Size()

	if err := c.sftpClient.MkdirAll(path.Dir(remotePath)); err != nil {
		return err
	}

	remoteFile, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	return copyWithProgress(remoteFile, localFile, total, onProgress, cancel)
}

// UploadPathWithProgress uploads either a file or a directory recursively.
func (c *Client) UploadPathWithProgress(localPath, remotePath string, onProgress ProgressFunc, cancel <-chan struct{}) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return c.UploadWithProgress(localPath, remotePath, onProgress, cancel)
	}

	entries, total, err := collectLocalEntries(localPath)
	if err != nil {
		return err
	}
	if err := c.sftpClient.MkdirAll(remotePath); err != nil {
		return err
	}

	var copiedTotal int64
	if onProgress != nil {
		onProgress(0, total)
	}

	for _, entry := range entries {
		select {
		case <-cancel:
			return errors.New("transfer cancelled")
		default:
		}

		rel := strings.TrimPrefix(entry.path, localPath)
		rel = strings.TrimPrefix(rel, string(filepath.Separator))
		remoteEntryPath := path.Join(remotePath, filepath.ToSlash(rel))

		if entry.isDir {
			if err := c.sftpClient.MkdirAll(remoteEntryPath); err != nil {
				return err
			}
			continue
		}

		if err := c.sftpClient.MkdirAll(path.Dir(remoteEntryPath)); err != nil {
			return err
		}

		localFile, err := os.Open(entry.path)
		if err != nil {
			return err
		}

		remoteFile, err := c.sftpClient.Create(remoteEntryPath)
		if err != nil {
			_ = localFile.Close()
			return err
		}

		progressFn := func(fileCopied int64, _ int64) {
			if onProgress != nil {
				onProgress(copiedTotal+fileCopied, total)
			}
		}
		copyErr := copyWithProgress(remoteFile, localFile, entry.size, progressFn, cancel)
		_ = remoteFile.Close()
		_ = localFile.Close()
		if copyErr != nil {
			return copyErr
		}

		copiedTotal += entry.size
		if onProgress != nil {
			onProgress(copiedTotal, total)
		}
	}

	return nil
}

func (c *Client) Download(remotePath, localPath string) error {
	remoteFile, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	_, err = io.Copy(localFile, remoteFile)
	return err
}

func (c *Client) DownloadWithProgress(remotePath, localPath string, onProgress ProgressFunc, cancel <-chan struct{}) error {
	remoteFile, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	remoteInfo, err := remoteFile.Stat()
	if err != nil {
		return err
	}
	total := remoteInfo.Size()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	return copyWithProgress(localFile, remoteFile, total, onProgress, cancel)
}

// DownloadPathWithProgress downloads either a file or a directory recursively.
func (c *Client) DownloadPathWithProgress(remotePath, localPath string, onProgress ProgressFunc, cancel <-chan struct{}) error {
	info, err := c.sftpClient.Stat(remotePath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return c.DownloadWithProgress(remotePath, localPath, onProgress, cancel)
	}

	entries, total := collectRemoteEntries(c.sftpClient, remotePath)
	if err := os.MkdirAll(localPath, 0o755); err != nil {
		return err
	}

	var copiedTotal int64
	if onProgress != nil {
		onProgress(0, total)
	}

	for _, entry := range entries {
		select {
		case <-cancel:
			return errors.New("transfer cancelled")
		default:
		}

		rel := strings.TrimPrefix(entry.path, remotePath)
		rel = strings.TrimPrefix(rel, "/")
		localEntryPath := filepath.Join(localPath, filepath.FromSlash(rel))

		if entry.isDir {
			if err := os.MkdirAll(localEntryPath, 0o755); err != nil {
				return err
			}
			continue
		}

		remoteFile, err := c.sftpClient.Open(entry.path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(localEntryPath), 0o755); err != nil {
			_ = remoteFile.Close()
			return err
		}
		localFile, err := os.Create(localEntryPath)
		if err != nil {
			_ = remoteFile.Close()
			return err
		}

		progressFn := func(fileCopied int64, _ int64) {
			if onProgress != nil {
				onProgress(copiedTotal+fileCopied, total)
			}
		}
		copyErr := copyWithProgress(localFile, remoteFile, entry.size, progressFn, cancel)
		_ = localFile.Close()
		_ = remoteFile.Close()
		if copyErr != nil {
			return copyErr
		}

		copiedTotal += entry.size
		if onProgress != nil {
			onProgress(copiedTotal, total)
		}
	}

	return nil
}

func (c *Client) Mkdir(path string) error {
	return c.sftpClient.Mkdir(path)
}

func (c *Client) Remove(path string) error {
	return c.sftpClient.Remove(path)
}

func (c *Client) Stat(path string) (os.FileInfo, error) {
	return c.sftpClient.Stat(path)
}

func (c *Client) PathSeparator() string {
	return "/"
}

func (c *Client) Pwd() (string, error) {
	return c.sftpClient.Getwd()
}

func (c *Client) OpenRead(remotePath string) (io.ReadCloser, error) {
	return c.sftpClient.Open(remotePath)
}

func (c *Client) Close() error {
	return c.sftpClient.Close()
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, onProgress ProgressFunc, cancel <-chan struct{}) error {
	buf := make([]byte, 32*1024)
	var copied int64

	if onProgress != nil {
		onProgress(0, total)
	}

	for {
		select {
		case <-cancel:
			return errors.New("transfer cancelled")
		default:
		}

		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			copied += int64(written)
			if onProgress != nil {
				onProgress(copied, total)
			}
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

type fsEntry struct {
	path  string
	isDir bool
	size  int64
}

func collectLocalEntries(root string) ([]fsEntry, int64, error) {
	var entries []fsEntry
	var total int64

	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		entry := fsEntry{path: p, isDir: info.IsDir()}
		if !info.IsDir() {
			entry.size = info.Size()
			total += info.Size()
		}
		entries = append(entries, entry)
		return nil
	})
	return entries, total, err
}

func collectRemoteEntries(client *sftp.Client, root string) ([]fsEntry, int64) {
	var entries []fsEntry
	var total int64

	walker := client.Walk(root)
	for walker.Step() {
		if walker.Err() != nil {
			continue
		}
		p := walker.Path()
		if p == root {
			continue
		}

		stat := walker.Stat()
		entry := fsEntry{
			path:  p,
			isDir: stat.IsDir(),
		}
		if !stat.IsDir() {
			entry.size = stat.Size()
			total += stat.Size()
		}
		entries = append(entries, entry)
	}

	return entries, total
}
