package sftp

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"

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
