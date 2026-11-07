package app

import "testing"

func TestOpenDatabaseAppliesMigrations(t *testing.T) {
	db, err := OpenDatabase("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'users'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("users table count = %d, want 1", count)
	}
}
