package sample_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	sample "github.com/shuymn-sandbox/dockertest-example/pkg/simple"
)

var db *sql.DB

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to create docker pool: %v", err)
	}

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get working directory: %v", err)
	}

	resource, err := pool.RunWithOptions(
		&dockertest.RunOptions{
			Repository: "mysql",
			Tag:        "8.0",
			Env: []string{
				"MYSQL_DATABASE=sample_db",
				"MYSQL_PASSWORD=password",
				"MYSQL_USER=user",
				"MYSQL_ROOT_PASSWORD=password",
			},
			Mounts: []string{
				fmt.Sprintf("%s:/docker-entrypoint-initdb.d/00_schema.sql", filepath.Join(pwd, "./../../schema.sql")),
				fmt.Sprintf("%s:/etc/mysql/conf.d/my.cnf:ro", filepath.Join(pwd, "./../../docker/mysql/conf.d/my_custom.cnf")),
			},
		},
		func(config *docker.HostConfig) {
			// コンテナが終了したら削除する
			config.AutoRemove = true
			// コンテナの起動に失敗してもリトライしない
			config.RestartPolicy = docker.RestartPolicy{Name: "no"}
		},
	)
	if err != nil {
		log.Fatalf("failed to run mysql container: %v", err)
	}

	config := &mysql.Config{
		User:                 "user",
		Passwd:               "password",
		Net:                  "tcp",
		Addr:                 resource.GetHostPort("3306/tcp"),
		DBName:               "sample_db",
		ParseTime:            true,
		AllowNativePasswords: true,
	}

	// MySQLの立ち上がりを待つ
	err = pool.Retry(func() error {
		m, retryErr := sql.Open("mysql", config.FormatDSN())
		if retryErr != nil {
			return retryErr
		}
		return m.Ping()
	})
	if err != nil {
		log.Fatalf("failed to connect to mysql: %v", err)
	}

	db, err = sql.Open("mysql", config.FormatDSN())
	if err != nil {
		log.Fatalf("failed to open mysql: %v", err)
	}

	// テストを実行
	code := m.Run()

	// コンテナを終了
	if err = pool.Purge(resource); err != nil {
		log.Fatalf("failed to purge mysql container: %v", err)
	}

	os.Exit(code)
}

func TestApp_CreateUser(t *testing.T) {
	ctx := context.Background()
	app := sample.New(db)

	if err := app.CreateUser(ctx, "shuymn", "test@shuymn.me"); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	row := db.QueryRowContext(ctx, "SELECT id, username, email FROM users WHERE username = ?", "shuymn")
	var (
		id              int
		username, email string
	)
	if err := row.Scan(&id, &username, &email); err != nil {
		t.Fatalf("failed to scan user: %v", err)
	}

	if id != 1 {
		t.Errorf("unexpected id: want %d, got %d", 1, id)
	}
	if username != "shuymn" {
		t.Errorf("unexpected username: want %s, got %s", "shuymn", username)
	}
	if email != "test@shuymn.me" {
		t.Errorf("unexpected email: want %s, got %s", "test@shuymn.me", email)
	}
}
