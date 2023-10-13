package testutil

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"

	"github.com/go-sql-driver/mysql"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.uber.org/multierr"
)

const mysqlExpireSeconds = 60

var mysqlSchemaPath string

// schema.sqlを見つける
func initMySQLSchemaPath() error {
	if mysqlSchemaPath != "" {
		return nil
	}

	var err error
	mysqlSchemaPath, err = findUpwardFileOrDirectory("schema.sql")
	return err
}

func TestWithMySQL(pool *dockertest.Pool) (_ *sql.DB, _ Cleanup, err error) {
	pool, err = initDockertest(pool)
	if err != nil {
		return nil, nil, err
	}

	if err = initMySQLSchemaPath(); err != nil {
		return nil, nil, err
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
				fmt.Sprintf("%s:/docker-entrypoint-initdb.d/00_schema.sql", mysqlSchemaPath),
				fmt.Sprintf("%s:/etc/mysql/conf.d/my.cnf:ro", filepath.Join(dockerDir, "mysql", "conf.d", "my_custom.cnf")),
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

	cleanup := func() error {
		if purgeErr := pool.Purge(resource); purgeErr != nil {
			return fmt.Errorf("failed to purge mysql container: %w", purgeErr)
		}
		return nil
	}
	defer func() {
		// この関数の中でエラーが発生したらcleanupを呼ぶ
		if err != nil {
			err = multierr.Append(err, cleanup())
		}
	}()

	// コンテナを停止するまでの時間を設定
	if err = resource.Expire(mysqlExpireSeconds); err != nil {
		log.Fatalf("failed to set expire time: %v", err)
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

	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		log.Fatalf("failed to open mysql: %v", err)
	}

	return db, cleanup, nil
}
