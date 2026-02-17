package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SodaItem struct {
	Name  string
	Price float32
}

type SnackItem struct {
	Name  string
	Price float32
}

type FridgeItem struct {
	Name  string
	Type  string
	Price float32
}

type fridgeData struct {
	Sodas  []SodaItem
	Snacks []SnackItem
}

func getSodas(db *pgxpool.Pool, ctx context.Context) ([]SodaItem, error) {
	sodaRows, err := db.Query(ctx, "SELECT name, price FROM sodaFridge WHERE type = 'soda'")
	if err != nil {
		return nil, err
	}
	defer sodaRows.Close()
	var sodas []SodaItem
	for sodaRows.Next() {
		var soda SodaItem
		if err := sodaRows.Scan(&soda.Name, &soda.Price); err != nil {
			slog.Error("Failed to scan soda item:", err)
			continue
		}
		sodas = append(sodas, soda)
	}

	return sodas, nil

}

func getSoda(db *pgxpool.Pool, ctx context.Context, name string) (*SodaItem, error) {
	var soda SodaItem
	err := db.QueryRow(ctx, "SELECT name, price FROM sodaFridge WHERE type = 'soda' AND name = $1", name).Scan(&soda.Name, &soda.Price)
	if err != nil {
		return nil, err
	}
	return &soda, nil
}

func getSnacks(db *pgxpool.Pool, ctx context.Context) ([]SnackItem, error) {
	snackRows, err := db.Query(ctx, "SELECT name, price FROM sodaFridge WHERE type = 'snack'")
	if err != nil {
		return nil, err
	}
	defer snackRows.Close()
	var snacks []SnackItem
	for snackRows.Next() {
		var snack SnackItem
		if err := snackRows.Scan(&snack.Name, &snack.Price); err != nil {
			slog.Error("Failed to scan snack item:", err)
			continue
		}
		snacks = append(snacks, snack)
	}

	return snacks, nil
}

func getSnack(db *pgxpool.Pool, ctx context.Context, name string) (*SnackItem, error) {
	var snack SnackItem
	err := db.QueryRow(ctx, "SELECT name, price FROM sodaFridge WHERE type = 'snack' AND name = $1", name).Scan(&snack.Name, &snack.Price)
	if err != nil {
		return nil, err
	}
	return &snack, nil
}

func getFridgeItem(db *pgxpool.Pool, ctx context.Context, name string) (FridgeItem, error) {
	var item FridgeItem
	err := db.QueryRow(ctx, "SELECT name, type, price FROM sodaFridge WHERE name = $1", name).Scan(&item.Name, &item.Type, &item.Price)
	if err != nil {
		return FridgeItem{}, err
	}
	return item, nil
}

func addFridgeItem(db *pgxpool.Pool, ctx context.Context, name, t string, price float32) error {
	_, err := db.Exec(ctx, "INSERT INTO sodaFridge (name, type, price) VALUES ($1, $2, $3)", name, t, price)
	return err
}

func updateFridgeItem(db *pgxpool.Pool, ctx context.Context, oldName, name string, price float32) error {
	_, err := db.Exec(ctx, "UPDATE sodaFridge SET name = $1, price = $2 WHERE name = $3", name, price, oldName)
	return err
}

func deleteFridgeItem(db *pgxpool.Pool, ctx context.Context, name string) error {
	_, err := db.Exec(ctx, "DELETE FROM sodaFridge WHERE name = $1", name)
	return err
}
