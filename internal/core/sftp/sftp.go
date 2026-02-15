package sftp

import (
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

func (c *Client) Close() error {
	return c.sftpClient.Close()
}
