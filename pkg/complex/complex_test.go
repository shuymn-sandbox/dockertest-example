package complex_test

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/shuymn-sandbox/dockertest-example/internal/testutil"
	pkgcomplex "github.com/shuymn-sandbox/dockertest-example/pkg/complex"
)

var (
	db       *sql.DB
	emulator *testutil.FirebaseEmulator
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatal(err)
	}

	var cleanupMySQL testutil.Cleanup
	db, cleanupMySQL, err = testutil.TestWithMySQL(pool)
	if err != nil {
		log.Fatal(err)
	}

	var cleanupFirebase testutil.Cleanup
	emulator, cleanupFirebase, err = testutil.TestWithFirebase(pool)
	if err != nil {
		log.Fatal(err)
	}

	code := m.Run()

	if err = cleanupMySQL(); err != nil {
		log.Fatal(err)
	}
	if err = cleanupFirebase(); err != nil {
		log.Fatal(err)
	}

	os.Exit(code)
}

func TestApp_CreateUser(t *testing.T) {
	firestore := emulator.SetupFirestore(t)

	ctx := context.Background()
	app := pkgcomplex.NewApp(db, firestore)

	_, err := app.CreateUser(ctx, "shuymn", "test@shuymn.me")
	if err != nil {
		t.Fatal(err)
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

func TestApp_SendMessage(t *testing.T) {
	firestore := emulator.SetupFirestore(t)

	ctx := context.Background()
	app := pkgcomplex.NewApp(db, firestore)

	user, err := app.CreateUser(ctx, "user", "user@shuymn.me")
	if err != nil {
		t.Fatal(err)
	}

	msg, err := app.SendMessage(ctx, user, "hello")
	if err != nil {
		t.Fatal(err)
	}

	doc, err := firestore.Collection("messages").Doc(msg.ID).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := doc.Data()

	if got["body"] != msg.Body {
		t.Errorf("unexpected body: want %s, got %s", msg.Body, got["body"])
	}
}
