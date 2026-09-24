package personal

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Footprints(ctx context.Context) (Footprints, error) {
	var data Footprints
	data.Locations = []Location{}
	data.Stays = []Stay{}
	data.Routes = []Route{}
	rows, err := s.pool.Query(ctx, `SELECT id,name,latitude,longitude,location_type,icon,sort_order
		FROM footprint_locations ORDER BY sort_order,id`)
	if err != nil {
		return Footprints{}, err
	}
	for rows.Next() {
		var item Location
		if err := rows.Scan(&item.ID, &item.Name, &item.Latitude, &item.Longitude, &item.Type, &item.Icon, &item.SortOrder); err != nil {
			rows.Close()
			return Footprints{}, err
		}
		data.Locations = append(data.Locations, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Footprints{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT id,start_date,end_date,is_present,location_id,title,stay_type,description,sort_order
		FROM footprint_stays ORDER BY sort_order,id`)
	if err != nil {
		return Footprints{}, err
	}
	for rows.Next() {
		var item Stay
		if err := rows.Scan(&item.ID, &item.StartDate, &item.EndDate, &item.IsPresent, &item.LocationID, &item.Title, &item.Type, &item.Description, &item.SortOrder); err != nil {
			rows.Close()
			return Footprints{}, err
		}
		data.Stays = append(data.Stays, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Footprints{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT id,from_location_id,to_location_id,route_date,transport,label,description,images,sort_order
		FROM footprint_routes ORDER BY sort_order,id`)
	if err != nil {
		return Footprints{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Route
		if err := rows.Scan(&item.ID, &item.From, &item.To, &item.Date, &item.Transport, &item.Label, &item.Description, &item.Images, &item.SortOrder); err != nil {
			return Footprints{}, err
		}
		if item.Images == nil {
			item.Images = []string{}
		}
		data.Routes = append(data.Routes, item)
	}
	return data, rows.Err()
}

func (s *Store) ReplaceFootprints(ctx context.Context, data Footprints) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return err
	}
	if err := replaceFootprints(ctx, tx, data); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ReplaceAll(ctx context.Context, footprints Footprints, timeline []Internship) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return err
	}
	if err := replaceFootprints(ctx, tx, footprints); err != nil {
		return err
	}
	if err := replaceTimeline(ctx, tx, timeline); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replaceFootprints(ctx context.Context, tx pgx.Tx, data Footprints) error {
	if _, err := tx.Exec(ctx, "DELETE FROM footprint_routes"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM footprint_stays"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM footprint_locations"); err != nil {
		return err
	}
	for index, item := range data.Locations {
		if _, err := tx.Exec(ctx, `INSERT INTO footprint_locations (id,name,latitude,longitude,location_type,icon,sort_order)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`, item.ID, item.Name, item.Latitude, item.Longitude, item.Type, item.Icon, index); err != nil {
			return err
		}
	}
	for index, item := range data.Stays {
		if _, err := tx.Exec(ctx, `INSERT INTO footprint_stays (id,start_date,end_date,is_present,location_id,title,stay_type,description,sort_order)
			VALUES (CASE WHEN $1='' THEN gen_random_uuid() ELSE $1::uuid END,$2,$3,$4,$5,$6,$7,$8,$9)`,
			item.ID, item.StartDate, item.EndDate, item.IsPresent, item.LocationID, item.Title, item.Type, item.Description, index); err != nil {
			return err
		}
	}
	for index, item := range data.Routes {
		images := item.Images
		if images == nil {
			images = []string{}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO footprint_routes (id,from_location_id,to_location_id,route_date,transport,label,description,images,sort_order)
			VALUES (CASE WHEN $1='' THEN gen_random_uuid() ELSE $1::uuid END,$2,$3,$4,$5,$6,$7,$8,$9)`,
			item.ID, item.From, item.To, item.Date, item.Transport, item.Label, item.Description, images, index); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Timeline(ctx context.Context) ([]Internship, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,start_date,end_date,is_present,company,icon,icon_color,position,description,sort_order
		FROM internship_experiences ORDER BY sort_order,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Internship, 0)
	for rows.Next() {
		var item Internship
		if err := rows.Scan(&item.ID, &item.StartDate, &item.EndDate, &item.IsPresent, &item.Company, &item.Icon, &item.IconColor, &item.Position, &item.Description, &item.SortOrder); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ReplaceTimeline(ctx context.Context, items []Internship) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := replaceTimeline(ctx, tx, items); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func replaceTimeline(ctx context.Context, tx pgx.Tx, items []Internship) error {
	if _, err := tx.Exec(ctx, "DELETE FROM internship_experiences"); err != nil {
		return err
	}
	for index, item := range items {
		if _, err := tx.Exec(ctx, `INSERT INTO internship_experiences (id,start_date,end_date,is_present,company,icon,icon_color,position,description,sort_order)
			VALUES (CASE WHEN $1='' THEN gen_random_uuid() ELSE $1::uuid END,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			item.ID, item.StartDate, item.EndDate, item.IsPresent, item.Company, item.Icon, item.IconColor, item.Position, item.Description, index); err != nil {
			return err
		}
	}
	return nil
}
