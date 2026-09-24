package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/config"
	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
	"github.com/SmileSnow819/YukiBloom-backend/internal/media"
	"github.com/SmileSnow819/YukiBloom-backend/internal/sitecontent"
	"github.com/jackc/pgx/v5"
)

type options struct {
	site         string
	translations string
	about        string
	music        string
	assets       string
	apply        bool
	replace      bool
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv, os.Stdout); err != nil {
		log.Printf("站点内容导入未完成：%v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, output io.Writer) error {
	flags := flag.NewFlagSet("import-site", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	settings := options{}
	flags.StringVar(&settings.site, "site", "../YukiBloom/config/site.yaml", "站点内容配置文件")
	flags.StringVar(&settings.translations, "translations", "../YukiBloom/config/i18n-content.yaml", "内容翻译文件")
	flags.StringVar(&settings.about, "about", "../YukiBloom/src/pages/about.astro", "关于我页面")
	flags.StringVar(&settings.music, "music", "../YukiBloom/src/pages/music.md", "歌单页面")
	flags.StringVar(&settings.assets, "assets", "../YukiBloom/public", "Astro public 目录")
	flags.BoolVar(&settings.apply, "apply", false, "执行导入；省略时只做预检查")
	flags.BoolVar(&settings.replace, "replace", false, "已有内容时允许替换")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(output, "用法：go run ./cmd/import-site [-site 文件] [-translations 文件] [-about 文件] [-music 文件] [-assets 目录] [-apply] [-replace]\n默认只做预检查；已有站点数据时需要显式添加 -replace 才会覆盖。")
			return writeErr
		}
		return errors.New("命令参数不正确，请检查站点内容导入参数")
	}
	content, pages, err := sitecontent.LoadLegacy(settings.site, settings.translations, settings.about, settings.music)
	if err != nil {
		return err
	}
	assets := contentAssets(&content)
	validated := make(map[string]struct{})
	for _, pointer := range assets {
		value := *pointer
		file, err := localAsset(settings.assets, value)
		if err != nil {
			return fmt.Errorf("图片 %s 的路径有误：%w", value, err)
		}
		if file == "" {
			continue
		}
		if _, exists := validated[value]; exists {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("读取图片 %s 失败：%w", value, err)
		}
		if _, err := media.Validate(data); err != nil {
			return fmt.Errorf("图片 %s 无效：%w", value, err)
		}
		validated[value] = struct{}{}
	}
	if _, err := fmt.Fprintf(output, "站点内容预检查完成：%d 个社交链接、%d 个精选分类、%d 个精选系列、%d 组歌单、%d 条内容翻译、%d 个独立页面、%d 张本地图片，未发现格式问题。\n", len(content.SocialLinks), len(content.FeaturedCategories), len(content.FeaturedSeries), len(content.MusicGroups), len(content.Translations), len(pages), len(validated)); err != nil {
		return err
	}
	if !settings.apply {
		_, err := fmt.Fprintln(output, "当前为预检查模式，未连接数据库，也未写入文件。确认内容后，添加 -apply 执行导入。")
		return err
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	startupCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	pool, err := database.Open(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("数据库连接失败：%w", err)
	}
	defer pool.Close()
	if err := database.Migrate(startupCtx, pool); err != nil {
		return fmt.Errorf("数据库迁移失败：%w", err)
	}
	var existing bool
	if err := pool.QueryRow(startupCtx, `SELECT EXISTS(SELECT 1 FROM site_profile) OR EXISTS(SELECT 1 FROM content_pages WHERE slug IN ('about','music'))`).Scan(&existing); err != nil {
		return fmt.Errorf("检查现有站点内容失败：%w", err)
	}
	if existing && !settings.replace {
		return errors.New("数据库已有站点资料或独立页面。为避免覆盖后台修改，确认替换后再添加 -replace")
	}
	mediaStore := media.NewStore(pool, cfg.UploadDir)
	imageURLs := make(map[string]string)
	newMedia := make([]string, 0)
	for _, pointer := range assets {
		value := *pointer
		file, err := localAsset(settings.assets, value)
		if err != nil || file == "" {
			continue
		}
		url, exists := imageURLs[value]
		if !exists {
			opened, err := os.Open(file)
			if err != nil {
				cleanupMedia(startupCtx, mediaStore, newMedia)
				return fmt.Errorf("打开图片失败：%w", err)
			}
			item, uploadErr := mediaStore.Save(startupCtx, opened)
			opened.Close()
			if uploadErr != nil {
				cleanupMedia(startupCtx, mediaStore, newMedia)
				return fmt.Errorf("导入图片 %s 失败：%w", value, uploadErr)
			}
			url = item.URL
			newMedia = append(newMedia, item.ID)
			imageURLs[value] = url
		}
		*pointer = url
	}
	store := sitecontent.NewStore(pool)
	if err := store.Replace(startupCtx, content); err != nil {
		cleanupMedia(startupCtx, mediaStore, newMedia)
		return fmt.Errorf("保存站点内容失败：%w", err)
	}
	for _, page := range pages {
		var id string
		var version int64
		err := pool.QueryRow(startupCtx, `SELECT id,version FROM content_pages WHERE locale=$1 AND slug=$2`, page.Locale, page.Slug).Scan(&id, &version)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("查询页面 %s 失败：%w", page.Slug, err)
		}
		if id != "" {
			page.ID = id
			page.Version = version
		}
		saved, err := store.SavePage(startupCtx, page, true)
		if err != nil {
			return fmt.Errorf("导入页面 %s 失败：%w", page.Slug, err)
		}
		if saved.Status != "published" {
			if _, err := store.SetPagePublished(startupCtx, saved.ID, true); err != nil {
				return fmt.Errorf("发布页面 %s 失败：%w", page.Slug, err)
			}
		}
	}
	_, err = fmt.Fprintln(output, "站点资料、内容翻译、音乐列表和独立页面已导入数据库。")
	return err
}

func contentAssets(content *sitecontent.Content) []*string {
	refs := make([]*string, 0)
	if content.Profile != nil {
		refs = append(refs, &content.Profile.Avatar, &content.Profile.DefaultOGImage)
	}
	for index := range content.FeaturedCategories {
		refs = append(refs, &content.FeaturedCategories[index].Image)
	}
	for index := range content.FeaturedSeries {
		refs = append(refs, &content.FeaturedSeries[index].Cover)
	}
	for index := range content.FriendLinks {
		refs = append(refs, &content.FriendLinks[index].Image)
	}
	return refs
}

func localAsset(root, reference string) (string, error) {
	if !strings.HasPrefix(reference, "/img/") {
		return "", nil
	}
	relative := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(reference, "/")))
	if !filepath.IsLocal(relative) {
		return "", errors.New("图片路径不安全")
	}
	return filepath.Join(root, relative), nil
}

func cleanupMedia(ctx context.Context, store *media.Store, ids []string) {
	for _, id := range ids {
		_ = store.DeleteUnused(ctx, id)
	}
}
