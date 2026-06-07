package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SodaItem struct {
	Name     string
	Price    float32
	Priority int
}

type SnackItem struct {
	Name     string
	Price    float32
	Priority int
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
	sodaRows, err := db.Query(ctx, "SELECT name, price FROM sodaFridge WHERE type = 'soda' ORDER BY priority")
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
	snackRows, err := db.Query(ctx, "SELECT name, price FROM sodaFridge WHERE type = 'snack' ORDER BY priority")
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
	_, err := db.Exec(ctx, "INSERT INTO sodaFridge (name, type, price, priority) VALUES ($1, $2, $3, (SELECT COALESCE(MAX(priority), 0) + 1 FROM sodaFridge WHERE type = $2))", name, t, price)
	return err
}

func updateFridgeItem(db *pgxpool.Pool, ctx context.Context, oldName, name string, price float32) error {
	_, err := db.Exec(ctx, "UPDATE sodaFridge SET name = $1, price = $2 WHERE name = $3", name, price, oldName)
	return err
}

func updateFridgeItemPriority(db *pgxpool.Pool, ctx context.Context, name string, priority int) error {
	_, err := db.Exec(ctx, "UPDATE sodaFridge SET priority = $1 WHERE name = $2", priority, name)
	return err
}

func deleteFridgeItem(db *pgxpool.Pool, ctx context.Context, name string) error {
	_, err := db.Exec(ctx, "DELETE FROM sodaFridge WHERE name = $1", name)
	return err
}

func addCleanPoint(db *pgxpool.Pool, ctx context.Context, kthid string, date pgtype.Date) error {
	_, err := db.Exec(ctx, "INSERT INTO cleanPoints (kthid, date) VALUES ($1, $2)", kthid, date)
	return err
}

func removeCleanPoint(db *pgxpool.Pool, ctx context.Context, kthid string, date pgtype.Date) error {
	_, err := db.Exec(ctx, "DELETE FROM cleanPoints WHERE kthid = $1 AND date = $2", kthid, date)
	return err
}

func getCleanPointsByDay(db *pgxpool.Pool, ctx context.Context, date pgtype.Date) ([]string, error) {
	rows, err := db.Query(ctx, "SELECT kthid FROM cleanPoints WHERE date = $1", date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var kthids []string
	for rows.Next() {
		var kthid string
		if err := rows.Scan(&kthid); err != nil {
			slog.Error("Failed to scan clean point:", err)
			continue
		}
		kthids = append(kthids, kthid)
	}

	return kthids, nil
}

func getCleanPointsByKthid(db *pgxpool.Pool, ctx context.Context, kthid string, from pgtype.Date, to pgtype.Date) ([]pgtype.Date, error) {
	slog.Info("Getting clean points for kthid", "kthid", kthid, "from", from, "to", to)
	rows, err := db.Query(ctx, "SELECT date FROM cleanPoints WHERE kthid = $1 AND date >= $2 AND date <= $3", kthid, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []pgtype.Date
	for rows.Next() {
		var date pgtype.Date
		if err := rows.Scan(&date); err != nil {
			slog.Error("Failed to scan clean point:", err)
			continue
		}
		dates = append(dates, date)
	}

	return dates, nil
}

type CleanerPoints struct {
	Kthid  string
	Points int
}

func getTop10CleanersWithPoints(db *pgxpool.Pool, ctx context.Context) ([]CleanerPoints, error) {
	rows, err := db.Query(ctx, "SELECT kthid, COUNT(*) AS points FROM cleanPoints GROUP BY kthid ORDER BY points DESC LIMIT 10")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cleaners []CleanerPoints
	for rows.Next() {
		var cleaner CleanerPoints
		if err := rows.Scan(&cleaner.Kthid, &cleaner.Points); err != nil {
			slog.Error("Failed to scan cleaner:", err)
			continue
		}
		cleaners = append(cleaners, cleaner)
	}

	return cleaners, nil
}

func getTop10CleanersWithPointsSince(db *pgxpool.Pool, ctx context.Context, date pgtype.Date) ([]CleanerPoints, error) {
	rows, err := db.Query(ctx, "SELECT kthid, COUNT(*) AS points FROM cleanPoints WHERE date >= $1 GROUP BY kthid ORDER BY points DESC LIMIT 10", date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cleaners []CleanerPoints
	for rows.Next() {
		var cleaner CleanerPoints
		if err := rows.Scan(&cleaner.Kthid, &cleaner.Points); err != nil {
			slog.Error("Failed to scan cleaner:", err)
			continue
		}
		cleaners = append(cleaners, cleaner)
	}

	return cleaners, nil
}

func getAllCleanersSince(db *pgxpool.Pool, ctx context.Context, date pgtype.Date) ([]CleanerPoints, error) {
	rows, err := db.Query(ctx, "SELECT kthid, COUNT(*) AS points FROM cleanPoints WHERE date >= $1 GROUP BY kthid ORDER BY points DESC", date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cleaners []CleanerPoints
	for rows.Next() {
		var cleaner CleanerPoints
		if err := rows.Scan(&cleaner.Kthid, &cleaner.Points); err != nil {
			slog.Error("Failed to scan cleaner:", err)
			continue
		}
		cleaners = append(cleaners, cleaner)
	}

	return cleaners, nil
}
