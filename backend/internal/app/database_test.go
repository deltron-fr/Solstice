package app

import (
	"database/sql"
	"testing"

	"github.com/pact-marketplace/pact/backend/migrations"
	"github.com/pressly/goose/v3"
)

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

func TestListingStorefrontMigrationAddsBackwardCompatibleDefaults(t *testing.T) {
	db, err := OpenDatabase("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("INSERT INTO users (name, email) VALUES ('Seller', 'storefront@example.test')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO listings (seller_id, title, price, currency) VALUES (1, 'Tea', 100, 'NGN')"); err != nil {
		t.Fatal(err)
	}
	var category, unit, location, imageURL string
	if err := db.QueryRow("SELECT category, unit, location, image_url FROM listings WHERE id = 1").Scan(&category, &unit, &location, &imageURL); err != nil {
		t.Fatal(err)
	}
	if category != "General" || unit != "item" || location != "" || imageURL != "" {
		t.Fatalf("migration defaults = (%q, %q, %q, %q)", category, unit, location, imageURL)
	}
	if _, err := db.Exec("UPDATE listings SET image_url = 'javascript:alert(1)' WHERE id = 1"); err == nil {
		t.Fatal("expected non-HTTPS image URL to violate the schema constraint")
	}
}

func TestMigrationsBackfillStableOrderChainReferences(t *testing.T) {
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	goose.SetBaseFS(migrations.Files)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(db, ".", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO users (name, email) VALUES ('Buyer', 'buyer@example.test'), ('Seller', 'seller@example.test')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO listings (seller_id, title, price, currency) VALUES (2, 'Tea', 100, 'USDC')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO orders (buyer_id, seller_id, listing_id, quantity, unit_price, total_amount) VALUES (1, 2, 1, 1, 100, 100)"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatal(err)
	}
	var reference []byte
	if err := db.QueryRow("SELECT chain_order_id FROM orders WHERE id = 1").Scan(&reference); err != nil {
		t.Fatal(err)
	}
	if len(reference) != 32 {
		t.Fatalf("backfilled chain_order_id length = %d, want 32", len(reference))
	}
}
