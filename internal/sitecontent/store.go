package sitecontent

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("页面不存在")
	ErrConflict = errors.New("内容已被其他操作修改，或标识已被占用")
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Get(ctx context.Context) (Content, error) {
	content := emptyContent()
	var profile Profile
	err := s.pool.QueryRow(ctx, `SELECT title,alternate,subtitle,name,description,avatar_url,show_logo,author,site_url,default_og_image,start_year,timezone,keywords FROM site_profile WHERE singleton=true`).Scan(
		&profile.Title, &profile.Alternate, &profile.Subtitle, &profile.Name, &profile.Description, &profile.Avatar, &profile.ShowLogo, &profile.Author, &profile.URL, &profile.DefaultOGImage, &profile.StartYear, &profile.Timezone, &profile.Keywords)
	if err == nil {
		content.Profile = &profile
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Content{}, err
	}
	if profile.Keywords == nil {
		profile.Keywords = []string{}
	}
	if content.Profile != nil {
		content.Profile.Keywords = profile.Keywords
	}
	rows, err := s.pool.Query(ctx, `SELECT platform,url,icon,color,enabled FROM social_links ORDER BY sort_order,platform`)
	if err != nil {
		return Content{}, err
	}
	for rows.Next() {
		var item SocialLink
		if err := rows.Scan(&item.Platform, &item.URL, &item.Icon, &item.Color, &item.Enabled); err != nil {
			rows.Close()
			return Content{}, err
		}
		content.SocialLinks = append(content.SocialLinks, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Content{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT category_name,slug FROM category_mappings ORDER BY sort_order,category_name`)
	if err != nil {
		return Content{}, err
	}
	for rows.Next() {
		var item CategoryMapping
		if err := rows.Scan(&item.Name, &item.Slug); err != nil {
			rows.Close()
			return Content{}, err
		}
		content.CategoryMappings = append(content.CategoryMappings, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Content{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT link,label,image,description,enabled FROM featured_categories ORDER BY sort_order,link`)
	if err != nil {
		return Content{}, err
	}
	for rows.Next() {
		var item FeaturedCategory
		if err := rows.Scan(&item.Link, &item.Label, &item.Image, &item.Description, &item.Enabled); err != nil {
			rows.Close()
			return Content{}, err
		}
		content.FeaturedCategories = append(content.FeaturedCategories, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Content{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT slug,category_name,label,full_name,description,cover,enabled,icon,highlight_on_home,links FROM featured_series ORDER BY sort_order,slug`)
	if err != nil {
		return Content{}, err
	}
	for rows.Next() {
		var item FeaturedSeries
		var links []byte
		if err := rows.Scan(&item.Slug, &item.CategoryName, &item.Label, &item.FullName, &item.Description, &item.Cover, &item.Enabled, &item.Icon, &item.HighlightOnHome, &links); err != nil {
			rows.Close()
			return Content{}, err
		}
		if err := json.Unmarshal(links, &item.Links); err != nil {
			rows.Close()
			return Content{}, err
		}
		content.FeaturedSeries = append(content.FeaturedSeries, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Content{}, err
	}
	rows.Close()
	content.Navigation, err = s.readNavigation(ctx)
	if err != nil {
		return Content{}, err
	}
	rows, err = s.pool.Query(ctx, `SELECT id,title,content,announcement_type,priority,color,COALESCE(publish_date::text,''),starts_at,ends_at,link_url,link_text,link_external,enabled FROM site_announcements ORDER BY priority DESC,publish_date DESC,id`)
	if err != nil {
		return Content{}, err
	}
	for rows.Next() {
		var item Announcement
		if err := rows.Scan(&item.ID, &item.Title, &item.Content, &item.Type, &item.Priority, &item.Color, &item.PublishDate, &item.StartDate, &item.EndDate, &item.Link.URL, &item.Link.Text, &item.Link.External, &item.Enabled); err != nil {
			rows.Close()
			return Content{}, err
		}
		content.Announcements = append(content.Announcements, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Content{}, err
	}
	rows.Close()
	err = s.pool.QueryRow(ctx, `SELECT title,subtitle,apply_title,apply_description,example_yaml FROM friend_settings WHERE singleton=true`).Scan(
		&content.FriendSettings.Title, &content.FriendSettings.Subtitle, &content.FriendSettings.ApplyTitle, &content.FriendSettings.ApplyDesc, &content.FriendSettings.ExampleYAML)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Content{}, err
	}
	rows, err = s.pool.Query(ctx, `SELECT id,site,url,owner,description,image,color,status FROM friend_links ORDER BY sort_order,site`)
	if err != nil {
		return Content{}, err
	}
	for rows.Next() {
		var item FriendLink
		if err := rows.Scan(&item.ID, &item.Site, &item.URL, &item.Owner, &item.Description, &item.Image, &item.Color, &item.Status); err != nil {
			rows.Close()
			return Content{}, err
		}
		content.FriendLinks = append(content.FriendLinks, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Content{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT locale,entity_type,entity_key,label,full_name,description FROM content_translations ORDER BY locale,entity_type,entity_key`)
	if err != nil {
		return Content{}, err
	}
	for rows.Next() {
		var item Translation
		if err := rows.Scan(&item.Locale, &item.EntityType, &item.EntityKey, &item.Label, &item.FullName, &item.Description); err != nil {
			rows.Close()
			return Content{}, err
		}
		content.Translations = append(content.Translations, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Content{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT id,title,enabled FROM music_groups ORDER BY sort_order,id`)
	if err != nil {
		return Content{}, err
	}
	for rows.Next() {
		var group MusicGroup
		if err := rows.Scan(&group.ID, &group.Title, &group.Enabled); err != nil {
			rows.Close()
			return Content{}, err
		}
		group.Links, err = s.readMusicLinks(ctx, group.ID)
		if err != nil {
			rows.Close()
			return Content{}, err
		}
		content.MusicGroups = append(content.MusicGroups, group)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Content{}, err
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT id,title,url,enabled FROM background_music_tracks ORDER BY sort_order,id`)
	if err != nil {
		return Content{}, err
	}
	for rows.Next() {
		var item BackgroundTrack
		if err := rows.Scan(&item.ID, &item.Title, &item.URL, &item.Enabled); err != nil {
			rows.Close()
			return Content{}, err
		}
		content.BackgroundMusic = append(content.BackgroundMusic, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Content{}, err
	}
	rows.Close()
	return content, nil
}

func emptyContent() Content {
	return Content{SocialLinks: []SocialLink{}, CategoryMappings: []CategoryMapping{}, FeaturedCategories: []FeaturedCategory{}, FeaturedSeries: []FeaturedSeries{}, Navigation: []NavigationItem{}, Announcements: []Announcement{}, FriendLinks: []FriendLink{}, Translations: []Translation{}, MusicGroups: []MusicGroup{}, BackgroundMusic: []BackgroundTrack{}}
}

func (s *Store) readNavigation(ctx context.Context) ([]NavigationItem, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,parent_id,name,name_key,path,icon,sort_order FROM site_navigation ORDER BY sort_order,id`)
	if err != nil {
		return nil, err
	}
	type node struct {
		id, parent string
		item       NavigationItem
		order      int
	}
	var nodes []node
	for rows.Next() {
		var item node
		var parent *string
		if err := rows.Scan(&item.id, &parent, &item.item.Name, &item.item.NameKey, &item.item.Path, &item.item.Icon, &item.order); err != nil {
			rows.Close()
			return nil, err
		}
		item.item.ID = item.id
		if parent != nil {
			item.parent = *parent
		}
		nodes = append(nodes, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	children := make(map[string][]node, len(nodes))
	for _, item := range nodes {
		children[item.parent] = append(children[item.parent], item)
	}
	var build func(parent string) []NavigationItem
	build = func(parent string) []NavigationItem {
		items := make([]NavigationItem, 0, len(children[parent]))
		for _, child := range children[parent] {
			item := child.item
			item.Children = build(child.id)
			items = append(items, item)
		}
		return items
	}
	return build(""), nil
}

func (s *Store) readMusicLinks(ctx context.Context, groupID string) ([]MusicLink, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,title,url FROM music_links WHERE group_id=$1 ORDER BY sort_order,id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := make([]MusicLink, 0)
	for rows.Next() {
		var link MusicLink
		if err := rows.Scan(&link.ID, &link.Title, &link.URL); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *Store) Replace(ctx context.Context, content Content) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return err
	}
	for _, table := range []string{"music_links", "music_groups", "background_music_tracks", "content_translations", "friend_links", "friend_settings", "site_announcements", "site_navigation", "featured_series", "featured_categories", "category_mappings", "social_links", "site_profile"} {
		if _, err := tx.Exec(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}
	if content.Profile != nil {
		profile := content.Profile
		keywords := profile.Keywords
		if keywords == nil {
			keywords = []string{}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO site_profile (singleton,title,alternate,subtitle,name,description,avatar_url,show_logo,author,site_url,default_og_image,start_year,timezone,keywords)
			VALUES (true,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, profile.Title, profile.Alternate, profile.Subtitle, profile.Name, profile.Description, profile.Avatar, profile.ShowLogo, profile.Author, profile.URL, profile.DefaultOGImage, profile.StartYear, profile.Timezone, keywords); err != nil {
			return err
		}
	}
	for index, item := range content.SocialLinks {
		if _, err := tx.Exec(ctx, `INSERT INTO social_links (platform,url,icon,color,sort_order,enabled) VALUES ($1,$2,$3,$4,$5,$6)`, item.Platform, item.URL, item.Icon, item.Color, index, item.Enabled); err != nil {
			return err
		}
	}
	for index, item := range content.CategoryMappings {
		if _, err := tx.Exec(ctx, `INSERT INTO category_mappings (category_name,slug,sort_order) VALUES ($1,$2,$3)`, item.Name, item.Slug, index); err != nil {
			return err
		}
	}
	for index, item := range content.FeaturedCategories {
		if _, err := tx.Exec(ctx, `INSERT INTO featured_categories (link,label,image,description,sort_order,enabled) VALUES ($1,$2,$3,$4,$5,$6)`, item.Link, item.Label, item.Image, item.Description, index, item.Enabled); err != nil {
			return err
		}
	}
	for index, item := range content.FeaturedSeries {
		links, err := json.Marshal(item.Links)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO featured_series (slug,category_name,label,full_name,description,cover,enabled,icon,highlight_on_home,links,sort_order)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, item.Slug, item.CategoryName, item.Label, item.FullName, item.Description, item.Cover, item.Enabled, item.Icon, item.HighlightOnHome, links, index); err != nil {
			return err
		}
	}
	for index, item := range content.Navigation {
		if err := insertNavigation(ctx, tx, item, "", index, 0); err != nil {
			return err
		}
	}
	for _, item := range content.Announcements {
		if _, err := tx.Exec(ctx, `INSERT INTO site_announcements (id,title,content,announcement_type,priority,color,publish_date,starts_at,ends_at,link_url,link_text,link_external,enabled)
			VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::date,$8,$9,$10,$11,$12,$13)`, item.ID, item.Title, item.Content, item.Type, item.Priority, item.Color, item.PublishDate, item.StartDate, item.EndDate, item.Link.URL, item.Link.Text, item.Link.External, item.Enabled); err != nil {
			return err
		}
	}
	settings := content.FriendSettings
	if _, err := tx.Exec(ctx, `INSERT INTO friend_settings (singleton,title,subtitle,apply_title,apply_description,example_yaml) VALUES (true,$1,$2,$3,$4,$5)`, settings.Title, settings.Subtitle, settings.ApplyTitle, settings.ApplyDesc, settings.ExampleYAML); err != nil {
		return err
	}
	for index, item := range content.FriendLinks {
		if _, err := tx.Exec(ctx, `INSERT INTO friend_links (id,site,url,owner,description,image,color,status,sort_order)
			VALUES (CASE WHEN $1='' THEN gen_random_uuid() ELSE $1::uuid END,$2,$3,$4,$5,$6,$7,$8,$9)`, item.ID, item.Site, item.URL, item.Owner, item.Description, item.Image, item.Color, item.Status, index); err != nil {
			return err
		}
	}
	for _, item := range content.Translations {
		if _, err := tx.Exec(ctx, `INSERT INTO content_translations (locale,entity_type,entity_key,label,full_name,description) VALUES ($1,$2,$3,$4,$5,$6)`, item.Locale, item.EntityType, item.EntityKey, item.Label, item.FullName, item.Description); err != nil {
			return err
		}
	}
	for index, group := range content.MusicGroups {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO music_groups (id,title,sort_order,enabled) VALUES (CASE WHEN $1='' THEN gen_random_uuid() ELSE $1::uuid END,$2,$3,$4) RETURNING id`, group.ID, group.Title, index, group.Enabled).Scan(&id); err != nil {
			return err
		}
		for linkIndex, link := range group.Links {
			if _, err := tx.Exec(ctx, `INSERT INTO music_links (id,group_id,title,url,sort_order) VALUES (CASE WHEN $1='' THEN gen_random_uuid() ELSE $1::uuid END,$2,$3,$4,$5)`, link.ID, id, link.Title, link.URL, linkIndex); err != nil {
				return err
			}
		}
	}
	for index, item := range content.BackgroundMusic {
		if _, err := tx.Exec(ctx, `INSERT INTO background_music_tracks (id,title,url,sort_order,enabled) VALUES (CASE WHEN $1='' THEN gen_random_uuid() ELSE $1::uuid END,$2,$3,$4,$5)`, item.ID, item.Title, item.URL, index, item.Enabled); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func insertNavigation(ctx context.Context, tx pgx.Tx, item NavigationItem, parent string, order, depth int) error {
	if depth > 4 {
		return errors.New("导航菜单最多支持四层嵌套")
	}
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO site_navigation (id,parent_id,name,name_key,path,icon,sort_order)
		VALUES (CASE WHEN $1='' THEN gen_random_uuid() ELSE $1::uuid END,NULLIF($2,'')::uuid,$3,$4,$5,$6,$7) RETURNING id`, item.ID, parent, item.Name, item.NameKey, item.Path, item.Icon, order).Scan(&id); err != nil {
		return err
	}
	for index, child := range item.Children {
		if err := insertNavigation(ctx, tx, child, id, index, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) PublicPage(ctx context.Context, locale, slug string) (Page, error) {
	var page Page
	err := s.pool.QueryRow(ctx, `SELECT id,locale,slug,title,description,body_markdown,status,version,updated_at FROM content_pages WHERE locale=$1 AND slug=$2 AND status='published'`, locale, slug).Scan(
		&page.ID, &page.Locale, &page.Slug, &page.Title, &page.Description, &page.BodyMarkdown, &page.Status, &page.Version, &page.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Page{}, ErrNotFound
	}
	return page, err
}

func (s *Store) AdminPages(ctx context.Context) ([]Page, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,locale,slug,title,description,body_markdown,status,version,updated_at FROM content_pages ORDER BY updated_at DESC,locale,slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pages := make([]Page, 0)
	for rows.Next() {
		var item Page
		if err := rows.Scan(&item.ID, &item.Locale, &item.Slug, &item.Title, &item.Description, &item.BodyMarkdown, &item.Status, &item.Version, &item.UpdatedAt); err != nil {
			return nil, err
		}
		pages = append(pages, item)
	}
	return pages, rows.Err()
}

func (s *Store) SavePage(ctx context.Context, page Page, published bool) (Page, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return Page{}, err
	}
	if page.ID == "" {
		status := "draft"
		var publishedAt *time.Time
		if published {
			status = "published"
			now := time.Now()
			publishedAt = &now
		}
		err = tx.QueryRow(ctx, `INSERT INTO content_pages (locale,slug,title,description,body_markdown,status,published_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id,status,version,updated_at`, page.Locale, page.Slug, page.Title, page.Description, page.BodyMarkdown, status, publishedAt).Scan(&page.ID, &page.Status, &page.Version, &page.UpdatedAt)
	} else {
		err = tx.QueryRow(ctx, `UPDATE content_pages SET locale=$2,slug=$3,title=$4,description=$5,body_markdown=$6,version=version+1,updated_at=now()
			WHERE id=$1 AND version=$7 RETURNING status,version,updated_at`, page.ID, page.Locale, page.Slug, page.Title, page.Description, page.BodyMarkdown, page.Version).Scan(&page.Status, &page.Version, &page.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if checkErr := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM content_pages WHERE id=$1)", page.ID).Scan(&exists); checkErr != nil {
				return Page{}, checkErr
			}
			if !exists {
				return Page{}, ErrNotFound
			}
			return Page{}, ErrConflict
		}
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return Page{}, ErrConflict
	}
	if err != nil {
		return Page{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Page{}, err
	}
	return page, nil
}

func (s *Store) AdminPageByID(ctx context.Context, id string) (Page, error) {
	var page Page
	err := s.pool.QueryRow(ctx, `SELECT id,locale,slug,title,description,body_markdown,status,version,updated_at FROM content_pages WHERE id=$1`, id).Scan(
		&page.ID, &page.Locale, &page.Slug, &page.Title, &page.Description, &page.BodyMarkdown, &page.Status, &page.Version, &page.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Page{}, ErrNotFound
	}
	return page, err
}

func (s *Store) SetPagePublished(ctx context.Context, id string, published bool) (Page, error) {
	status := "draft"
	if published {
		status = "published"
	}
	var page Page
	err := s.pool.QueryRow(ctx, `UPDATE content_pages SET status=$2,
		published_at=CASE WHEN $2='published' THEN COALESCE(published_at,now()) ELSE NULL END,
		version=version+1,updated_at=now() WHERE id=$1 RETURNING id,locale,slug,title,description,body_markdown,status,version,updated_at`, id, status).Scan(
		&page.ID, &page.Locale, &page.Slug, &page.Title, &page.Description, &page.BodyMarkdown, &page.Status, &page.Version, &page.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Page{}, ErrNotFound
	}
	return page, err
}

func (s *Store) DeletePage(ctx context.Context, id string) error {
	result, err := s.pool.Exec(ctx, "DELETE FROM content_pages WHERE id=$1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
