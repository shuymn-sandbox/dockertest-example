package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/jaevor/go-nanoid"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.uber.org/multierr"
)

const firebaseExpireSeconds = 60

const (
	containerNameCharacters   = "abcdefghijklmnopqrstuvwxyz"
	containerNameNanoIDLength = 16
)

type FirebaseEmulator struct {
	ctx           context.Context
	firestoreHost string
	app           *firebase.App
}

func TestWithFirebase(pool *dockertest.Pool) (_ *FirebaseEmulator, _ Cleanup, err error) {
	pool, err = initDockertest(pool)
	if err != nil {
		return nil, nil, err
	}

	firebaseDockerPath := filepath.Join(dockerDir, "firebase")
	firebasercPath := filepath.Join(firebaseDockerPath, ".firebaserc")

	// firebaseの設定ファイルを読み込む
	f, err := os.Open(firebasercPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open %s: %w", firebasercPath, err)
	}
	defer func() {
		err = multierr.Append(err, f.Close())
	}()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", firebasercPath, err)
	}
	var firebaserc struct {
		Projects struct {
			Default string `json:"default"`
		} `json:"projects"`
	}
	if err = json.Unmarshal(b, &firebaserc); err != nil {
		return nil, nil, fmt.Errorf("could not parse .firebaserc: %w", err)
	}

	// ランダムなコンテナ名を生成する
	generateID, err := nanoid.CustomASCII(containerNameCharacters, containerNameNanoIDLength)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate container name: %w", err)
	}

	resource, err := pool.BuildAndRunWithOptions(
		filepath.Join(firebaseDockerPath, "Dockerfile"),
		&dockertest.RunOptions{
			Name:        fmt.Sprintf("dockertest-example-firebase_%s", generateID()),
			CapAdd:      []string{"SYS_PTRACE"},
			Cmd:         []string{"firebase", "emulators:start"},
			SecurityOpt: []string{"seccomp:unconfined"},
			Tty:         true,
			Mounts: []string{
				fmt.Sprintf("%s:/root/.cache:cached", filepath.Join(firebaseDockerPath, "bin")),
				fmt.Sprintf("%s:/root/.config:cached", filepath.Join(firebaseDockerPath, "config")),
				fmt.Sprintf("%s:/opt/.firebaserc", firebasercPath),
				fmt.Sprintf("%s:/opt/firebase.json", filepath.Join(firebaseDockerPath, "firebase.json")),
			},
			ExposedPorts: []string{"4000/tcp", "8000/tcp", "9000/tcp", "9099/tcp"},
		},
		func(config *docker.HostConfig) {
			// config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{Name: "no"}
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not start firebase emulator: %w", err)
	}

	cleanup := func() error {
		if purgeErr := pool.Purge(resource); purgeErr != nil {
			return fmt.Errorf("could not purge firebase emulator: %w", purgeErr)
		}
		return nil
	}
	defer func() {
		// この関数の中でエラーが発生したらcleanupを呼ぶ
		if err != nil {
			err = multierr.Append(err, cleanup())
		}
	}()

	if err = resource.Expire(firebaseExpireSeconds); err != nil {
		return nil, nil, fmt.Errorf("could not set expiration time for firebase emulator: %w", err)
	}

	healthcheck, err := url.JoinPath("http://", resource.GetHostPort("4000/tcp"), "emulator")
	if err != nil {
		return nil, nil, fmt.Errorf("could not create firebase emulator healthcheck url: %w", err)
	}

	ctx := context.Background()
	err = pool.Retry(func() error {
		req, retryErr := http.NewRequestWithContext(ctx, http.MethodGet, healthcheck, http.NoBody)
		if retryErr != nil {
			return retryErr
		}
		resp, retryErr := http.DefaultClient.Do(req)
		if retryErr != nil {
			return retryErr
		}
		if resp.Body == nil {
			return nil
		}
		// resp.Bodyは全部読み切るのが作法なので、読み切ってから閉じる
		_, retryErr = io.Copy(io.Discard, resp.Body)
		retryErr = multierr.Append(retryErr, retryErr)
		return multierr.Append(retryErr, resp.Body.Close())
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not connect to firebase emulator: %w", err)
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: firebaserc.Projects.Default,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not create firebase app: %w", err)
	}
	emulator := &FirebaseEmulator{
		ctx:           ctx,
		firestoreHost: resource.GetHostPort("8000/tcp"),
		app:           app,
	}

	return emulator, cleanup, nil
}

func (e *FirebaseEmulator) SetupFirestore(t *testing.T) *firestore.Client {
	t.Helper()
	// これが設定されているとFirestoreのクライアントがエミュレータに接続する
	t.Setenv("FIRESTORE_EMULATOR_HOST", e.firestoreHost)

	client, err := e.app.Firestore(e.ctx)
	if err != nil {
		t.Fatalf("could not create firestore client: %v", err)
	}
	return client
}
