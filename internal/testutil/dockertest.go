package testutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ory/dockertest/v3"
)

type Cleanup func() error

const maxUpwardTraversal = 10

var (
	dockerDir string

	errFileOrDirectoryNotFound = fmt.Errorf("file or directory not found")
)

func initDockertest(pool *dockertest.Pool) (*dockertest.Pool, error) {
	if pool == nil {
		var err error
		pool, err = dockertest.NewPool("")
		if err != nil {
			return nil, fmt.Errorf("could not construct pool: %w", err)
		}
	}

	if err := pool.Client.Ping(); err != nil {
		return nil, fmt.Errorf("could not connect to Docker: %w", err)
	}

	if err := initDockerDir(); err != nil {
		return nil, fmt.Errorf("could not initialize docker directory: %w", err)
	}

	return pool, nil
}

// docker ディレクトリのパスを取得してパッケージ変数に割り当てる
func initDockerDir() error {
	if dockerDir != "" {
		return nil
	}

	var err error
	dockerDir, err = findUpwardFileOrDirectory("docker")
	return err
}

// 現在のディレクトリから上方向に向かってファイルまたはディレクトリを探す
func findUpwardFileOrDirectory(s string) (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory: %w", err)
	}

	for i := 0; i < maxUpwardTraversal; i++ {
		path := filepath.Join(currentDir, s)
		if _, err = os.Stat(path); err == nil {
			return path, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("could not stat %s: %w", path, err)
		}
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		currentDir = parentDir
	}
	return "", errFileOrDirectoryNotFound
}
