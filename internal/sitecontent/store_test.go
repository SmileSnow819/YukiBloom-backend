package sitecontent

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSiteContentRoundTripAndPagePublishing(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("未设置 TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		current, err := store.Get(cleanupCtx)
		if err == nil {
			_ = store.Replace(cleanupCtx, Content{Version: current.Version})
		}
	})
	current, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	content := Content{
		Version:            current.Version,
		Profile:            &Profile{Title: "测试站点", Name: "站长", URL: "https://example.com", StartYear: 2026, Timezone: "Asia/Shanghai", Keywords: []string{"博客"}},
		SocialLinks:        []SocialLink{{Platform: "github", URL: "https://github.com/example", Icon: "ri:github-fill", Enabled: true}},
		CategoryMappings:   []CategoryMapping{{Name: "随笔", Slug: "essay"}},
		FeaturedCategories: []FeaturedCategory{{Link: "essay", Label: "随笔", Enabled: true}},
		FeaturedSeries:     []FeaturedSeries{{Slug: "weekly", CategoryName: "周刊", Enabled: true, HighlightOnHome: true, Links: map[string]string{"rss": "/rss.xml"}}},
		Navigation:         []NavigationItem{{Name: "文章", Children: []NavigationItem{{Name: "分类", Path: "/categories"}}}},
		MusicGroups:        []MusicGroup{{Title: "我的歌单", Enabled: true, Links: []MusicLink{{URL: "https://music.163.com/playlist?id=1"}}}},
	}
	if err := store.Replace(ctx, content); err != nil {
		t.Fatalf("保存站点内容失败：%v", err)
	}
	loaded, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("读取站点内容失败：%v", err)
	}
	if loaded.Profile == nil || loaded.Profile.Title != "测试站点" || len(loaded.Navigation) != 1 || len(loaded.Navigation[0].Children) != 1 || len(loaded.MusicGroups) != 1 || len(loaded.MusicGroups[0].Links) != 1 {
		t.Fatalf("站点内容往返后结构不正确：%+v", loaded)
	}
	if loaded.Version != content.Version+1 {
		t.Fatalf("保存后版本号应增加，实际为 %d", loaded.Version)
	}
	if err := store.Replace(ctx, content); !errors.Is(err, ErrConflict) {
		t.Fatalf("旧版本站点内容应被拒绝，实际错误为 %v", err)
	}
	if err := store.Import(ctx, Content{Version: loaded.Version, Profile: &Profile{Title: "不应保留"}}, []Page{
		{Locale: "zh-CN", Slug: "rollback-test", Title: "第一页面"},
		{Locale: "zh-CN", Slug: "", Title: "无效页面"},
	}); err == nil {
		t.Fatal("无效页面应使整次导入失败")
	}
	afterFailedImport, err := store.Get(ctx)
	if err != nil || afterFailedImport.Version != loaded.Version || afterFailedImport.Profile.Title != "测试站点" {
		t.Fatalf("导入失败后站点内容应回滚：内容 %+v，错误 %v", afterFailedImport, err)
	}
	if _, err := store.PublicPage(ctx, "zh-CN", "rollback-test"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("导入失败后页面应回滚，实际错误 %v", err)
	}
	page, err := store.SavePage(ctx, Page{Locale: "zh-CN", Slug: "about-test", Title: "关于测试", BodyMarkdown: "你好"}, false)
	if err != nil {
		t.Fatalf("创建页面草稿失败：%v", err)
	}
	defer store.DeletePage(context.Background(), page.ID)
	if _, err := store.PublicPage(ctx, "zh-CN", "about-test"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("页面草稿不应公开，实际错误为 %v", err)
	}
	if _, err := store.SetPagePublished(ctx, page.ID, true); err != nil {
		t.Fatalf("发布页面失败：%v", err)
	}
	if _, err := store.PublicPage(ctx, "zh-CN", "about-test"); err != nil {
		t.Fatalf("发布后的页面应可读取：%v", err)
	}
}

func TestGetMusicWithOneDatabaseConnection(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("未设置 TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	current, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		latest, getErr := store.Get(cleanupCtx)
		if getErr == nil {
			_ = store.Replace(cleanupCtx, Content{Version: latest.Version})
		}
	}()
	if err := store.Replace(ctx, Content{Version: current.Version, MusicGroups: []MusicGroup{{Title: "测试歌单", Enabled: true, Links: []MusicLink{{URL: "https://example.com/song"}}}}}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.MusicGroups) != 1 || len(loaded.MusicGroups[0].Links) != 1 {
		t.Fatalf("单连接读取歌单失败：%+v", loaded.MusicGroups)
	}
}
