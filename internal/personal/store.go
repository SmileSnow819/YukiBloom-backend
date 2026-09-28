package personal

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

var ErrConflict = errors.New("个人内容已被其他操作修改，请刷新后重试")

// NewStore 创建使用指定 PostgreSQL 连接池的个人内容存储。
// 参数：pool 是数据库连接池。
// 返回：配置好的 Store。
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Footprints 查询地点、停留和路线，并组装成足迹响应。
// 参数：s 是个人内容存储；ctx 控制数据库查询的取消和截止时间。
// 返回：Footprints 是完整足迹数据；error 表示查询或组装失败。
func (s *Store) Footprints(ctx context.Context) (Footprints, error) {
	var data Footprints
	if err := s.pool.QueryRow(ctx, `SELECT footprints_version FROM personal_content_revision WHERE singleton=true`).Scan(&data.Version); err != nil {
		return Footprints{}, err
	}
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

// ReplaceFootprints 在单个事务中替换足迹地点、停留和路线。
// 参数：s 是个人内容存储；ctx 控制数据库事务；data 是待保存的完整足迹数据。
// 返回：error；事务成功时返回 nil，数据库操作失败时返回错误。
func (s *Store) ReplaceFootprints(ctx context.Context, data Footprints) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE personal_content_revision SET footprints_version=footprints_version+1 WHERE singleton=true AND footprints_version=$1`, data.Version)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrConflict
	}
	if err := replaceFootprints(ctx, tx, data); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReplaceAll 在同一事务中替换足迹与实习经历。
// 参数：s 是个人内容存储；ctx 控制数据库事务；footprints 是完整足迹数据；timeline 是完整实习经历列表。
// 返回：error；事务成功时返回 nil，任一写入失败时返回错误并回滚。
func (s *Store) ReplaceAll(ctx context.Context, footprints Footprints, timeline []Internship) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE personal_content_revision SET footprints_version=footprints_version+1,timeline_version=timeline_version+1 WHERE singleton=true`); err != nil {
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

// replaceFootprints 使用现有事务替换足迹地点、停留和路线记录。
// 参数：ctx 控制数据库操作；tx 是调用方创建的事务；data 是完整足迹数据。
// 返回：error；全部写入成功时返回 nil，失败时返回数据库错误。
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

// Timeline 按展示顺序查询实习经历。
// 参数：s 是个人内容存储；ctx 控制数据库查询。
// 返回：[]Internship 是经历列表；int64 是当前版本号；error 表示查询或读取失败。
func (s *Store) Timeline(ctx context.Context) ([]Internship, int64, error) {
	var version int64
	if err := s.pool.QueryRow(ctx, `SELECT timeline_version FROM personal_content_revision WHERE singleton=true`).Scan(&version); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,start_date,end_date,is_present,company,icon,icon_color,position,description,sort_order
		FROM internship_experiences ORDER BY sort_order,id`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Internship, 0)
	for rows.Next() {
		var item Internship
		if err := rows.Scan(&item.ID, &item.StartDate, &item.EndDate, &item.IsPresent, &item.Company, &item.Icon, &item.IconColor, &item.Position, &item.Description, &item.SortOrder); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, version, rows.Err()
}

// ReplaceTimeline 在单个事务中替换全部实习经历。
// 参数：s 是个人内容存储；ctx 控制数据库事务；items 是待保存的完整经历列表；version 是读取时的版本号。
// 返回：error；事务成功时返回 nil，数据库操作失败时返回错误。
func (s *Store) ReplaceTimeline(ctx context.Context, items []Internship, version int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE personal_content_revision SET timeline_version=timeline_version+1 WHERE singleton=true AND timeline_version=$1`, version)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrConflict
	}
	if err := replaceTimeline(ctx, tx, items); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// replaceTimeline 使用现有事务替换实习经历记录。
// 参数：ctx 控制数据库操作；tx 是调用方创建的事务；items 是待保存的完整经历列表。
// 返回：error；全部写入成功时返回 nil，失败时返回数据库错误。
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
