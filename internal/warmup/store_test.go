package warmup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteFileURI(t *testing.T) {
	for path, want := range map[string]string{
		"C:/Users/runner/state/warmup.db": "file:///C:/Users/runner/state/warmup.db",
		"D:/state #1/warmup.db":           "file:///D:/state%20%231/warmup.db",
		"/tmp/state #1/warmup.db":         "file:///tmp/state%20%231/warmup.db",
	} {
		if got := sqliteFileURI(path); got != want {
			t.Errorf("SQLite URI for %q: got %q, want %q", path, got, want)
		}
	}
}

func TestStorePathWithSpacesAndHash(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state #1")
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	config, err := store.Config()
	if err == nil {
		config.Settings.Paused = true
		err = store.Save(config)
	}
	closeErr := store.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("persist config: %v, close: %v", err, closeErr)
	}
	if _, err := os.Stat(filepath.Join(dir, "warmup.db")); err != nil {
		t.Fatal(err)
	}
	store, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	config, err = store.Config()
	if err != nil || !config.Settings.Paused {
		t.Fatalf("config did not survive reopening: %+v, %v", config, err)
	}
}
