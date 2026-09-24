package sitecontent

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
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
		_ = store.Replace(cleanupCtx, Content{})
	})
	content := Content{
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
