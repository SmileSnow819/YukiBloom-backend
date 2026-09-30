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

// main 执行旧站点内容导入命令，并报告执行失败。
// 参数：无。
// 返回：无。
func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv, os.Stdout); err != nil {
		log.Printf("站点内容导入未完成：%v", err)
		os.Exit(1)
	}
}

// run 预检查旧站点内容并在指定 -apply 时事务化导入数据库。
// 参数：ctx 控制导入过程；args 是命令行参数；getenv 读取数据库等配置；output 接收检查结果。
// 返回：error；参数、旧文件、图片、数据库或输出处理失败时返回错误。
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
	if _, err := fmt.Fprintf(output, "站点内容预检查完成：%d 个社交链接、%d 个分类（%d 个在首页展示）、%d 个精选系列、%d 组歌单、%d 条内容翻译、%d 个独立页面、%d 张本地图片，未发现格式问题。\n", len(content.SocialLinks), len(content.Categories), countHomeCategories(content.Categories), len(content.FeaturedSeries), len(content.MusicGroups), len(content.Translations), len(pages), len(validated)); err != nil {
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
	if err := pool.QueryRow(startupCtx, `SELECT version > 0 OR EXISTS(SELECT 1 FROM content_pages WHERE slug IN ('about','music')) FROM site_content_revision WHERE singleton=true`).Scan(&existing); err != nil {
		return fmt.Errorf("检查现有站点内容失败：%w", err)
	}
	if existing && !settings.replace {
		return errors.New("数据库已有站点资料或独立页面。为避免覆盖后台修改，确认替换后再添加 -replace")
	}
	store := sitecontent.NewStore(pool)
	current, err := store.Get(startupCtx)
	if err != nil {
		return fmt.Errorf("读取站点内容版本失败：%w", err)
	}
	content.Version = current.Version
	mediaStorage, err := media.NewStorage(cfg)
	if err != nil {
		return fmt.Errorf("图片存储初始化失败：%w", err)
	}
	mediaStore := media.NewStore(pool, mediaStorage)
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
	if err := store.Import(startupCtx, content, pages); err != nil {
		cleanupMedia(startupCtx, mediaStore, newMedia)
		return fmt.Errorf("保存站点内容和独立页面失败：%w", err)
	}
	_, err = fmt.Fprintln(output, "站点资料、内容翻译、音乐列表和独立页面已导入数据库。")
	return err
}

// contentAssets 返回站点资料中所有可迁移图片地址的引用。
// 参数：content 是待迁移的站点内容；返回的指针用于将旧地址替换为上传后的地址。
// 返回：[]*string 是头像、精选图片和友链图片地址的引用列表。
func contentAssets(content *sitecontent.Content) []*string {
	refs := make([]*string, 0)
	if content.Profile != nil {
		refs = append(refs, &content.Profile.Avatar, &content.Profile.DefaultOGImage)
	}
	for index := range content.Categories {
		refs = append(refs, &content.Categories[index].Image)
	}
	for index := range content.FeaturedSeries {
		refs = append(refs, &content.FeaturedSeries[index].Cover)
	}
	for index := range content.FriendLinks {
		refs = append(refs, &content.FriendLinks[index].Image)
	}
	return refs
}

// 统计需要显示在首页的分类数量。
// 参数：categories 是待统计的分类列表。
// 返回：int 是启用首页展示的分类数量。
func countHomeCategories(categories []sitecontent.Category) int {
	count := 0
	for _, category := range categories {
		if category.ShowOnHome {
			count++
		}
	}
	return count
}

// localAsset 将旧站点内的图片 URL 转换为安全的本地文件路径。
// 参数：root 是旧站点资源目录；reference 是内容中的图片地址。
// 返回：string 是本地文件路径，非站内图片返回空字符串；error 表示路径不安全。
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

// cleanupMedia 回收导入失败后未被内容引用的新图片。
// 参数：ctx 控制数据库清理；store 是图片存储；ids 是本次导入生成的图片编号。
// 返回：无。
func cleanupMedia(ctx context.Context, store *media.Store, ids []string) {
	for _, id := range ids {
		_ = store.DeleteUnused(ctx, id)
	}
}
