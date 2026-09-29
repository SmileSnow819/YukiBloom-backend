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
	"sort"
	"strings"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/config"
	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
	"github.com/SmileSnow819/YukiBloom-backend/internal/importer"
	"github.com/SmileSnow819/YukiBloom-backend/internal/media"
	"github.com/SmileSnow819/YukiBloom-backend/internal/posts"
)

type options struct {
	source string
	assets string
	locale string
	apply  bool
}

type candidate struct {
	post      importer.MarkdownPost
	coverFile string
}

// main 执行旧文章导入命令，并报告执行失败。
// 参数：无。
// 返回：无。
func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv, os.Stdout); err != nil {
		log.Printf("导入未完成：%v", err)
		os.Exit(1)
	}
}

// run 检查旧 Markdown 文章及封面，并在指定 -apply 时写入数据库。
// 参数：ctx 控制导入过程；args 是命令行参数；getenv 读取数据库等配置；output 接收检查结果。
// 返回：error；参数、文章、图片、数据库或输出处理失败时返回错误。
func run(ctx context.Context, args []string, getenv func(string) string, output io.Writer) error {
	flags := flag.NewFlagSet("import-posts", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	settings := options{}
	flags.StringVar(&settings.source, "source", "../YukiBloom/src/content/blog", "Markdown 文章目录")
	flags.StringVar(&settings.assets, "assets", "../YukiBloom/public", "Astro public 目录")
	flags.StringVar(&settings.locale, "locale", "zh-CN", "导入文章的语言代码")
	flags.BoolVar(&settings.apply, "apply", false, "执行导入；省略时只做预检查")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(output, "用法：go run ./cmd/import-posts [-source 文章目录] [-assets 图片目录] [-locale 语言代码] [-apply]\n默认只做预检查；添加 -apply 后连接数据库并执行导入。")
			return writeErr
		}
		return errors.New("命令参数不正确，请检查 -source、-assets、-locale 和 -apply 参数")
	}
	paths, err := markdownFiles(settings.source)
	if err != nil {
		return fmt.Errorf("读取文章目录失败：%w", err)
	}
	candidates := make([]candidate, 0, len(paths))
	problems := 0
	seen := make(map[string]string)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(output, "文件 %s：读取失败：%v\n", path, err)
			problems++
			continue
		}
		post, err := importer.ParseMarkdown(path, settings.locale, data)
		if err != nil {
			fmt.Fprintf(output, "文件 %s：%v\n", path, err)
			problems++
			continue
		}
		if err := posts.ValidateInput(post.Input); err != nil {
			fmt.Fprintf(output, "文件 %s：文章字段无效：%v\n", path, err)
			problems++
			continue
		}
		key := post.Input.Locale + ":" + post.Input.Slug
		if first, duplicate := seen[key]; duplicate {
			fmt.Fprintf(output, "文件 %s：链接地址与 %s 重复（%s）\n", path, first, post.Input.Slug)
			problems++
			continue
		}
		seen[key] = path
		coverFile, err := resolveCover(settings.assets, post.CoverPath)
		if err != nil {
			fmt.Fprintf(output, "文件 %s：%v\n", path, err)
			problems++
			continue
		}
		if coverFile != "" {
			cover, err := os.ReadFile(coverFile)
			if err != nil {
				fmt.Fprintf(output, "文件 %s：封面读取失败：%v\n", path, err)
				problems++
				continue
			}
			if _, err := media.Validate(cover); err != nil {
				fmt.Fprintf(output, "文件 %s：封面图片无效：%v\n", path, err)
				problems++
				continue
			}
		}
		candidates = append(candidates, candidate{post: post, coverFile: coverFile})
	}
	if _, err := fmt.Fprintf(output, "预检查完成：扫描 %d 篇，符合格式 %d 篇，发现问题 %d 篇。\n", len(paths), len(candidates), problems); err != nil {
		return err
	}
	if problems > 0 {
		return fmt.Errorf("请先处理以上 %d 个问题，再重新运行预检查", problems)
	}
	if !settings.apply {
		_, err := fmt.Fprintln(output, "当前为预检查模式，未连接数据库，也未写入文件。确认内容后，添加 -apply 执行导入。")
		return err
	}
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	startupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	pool, err := database.Open(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("数据库连接失败：%w", err)
	}
	defer pool.Close()
	if err := database.Migrate(startupCtx, pool); err != nil {
		return fmt.Errorf("数据库迁移失败：%w", err)
	}
	mediaStorage, err := media.NewStorage(cfg)
	if err != nil {
		return fmt.Errorf("图片存储初始化失败：%w", err)
	}
	mediaStore := media.NewStore(pool, mediaStorage)
	importCtx, importCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer importCancel()
	tx, err := pool.Begin(importCtx)
	if err != nil {
		return fmt.Errorf("开始文章导入事务失败：%w", err)
	}
	committed := false
	newObjectKeys := make([]string, 0)
	defer func() {
		if committed {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = tx.Rollback(cleanupCtx)
		for _, key := range newObjectKeys {
			if err := mediaStore.RemoveObject(cleanupCtx, key); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Printf("清理未完成导入的图片失败：%v", err)
			}
		}
	}()
	if _, err := tx.Exec(importCtx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return fmt.Errorf("锁定文章导入事务失败：%w", err)
	}
	postStore := posts.NewStore(pool)
	coverIDs := make(map[string]string)
	created, skipped := 0, 0
	messages := make([]string, 0, len(candidates))
	for _, item := range candidates {
		if exists, err := postStore.ExistsInTx(importCtx, tx, item.post.Input.Locale, item.post.Input.Slug); err != nil {
			return fmt.Errorf("检查文章 %s 失败：%w", item.post.Input.Slug, err)
		} else if exists {
			messages = append(messages, fmt.Sprintf("已跳过 %s：数据库中已有此文章", item.post.Input.Slug))
			skipped++
			continue
		}
		if item.coverFile != "" {
			id, exists := coverIDs[item.post.CoverPath]
			if !exists {
				file, err := os.Open(item.coverFile)
				if err != nil {
					return fmt.Errorf("打开封面图片失败：%w", err)
				}
				image, key, uploadErr := mediaStore.SaveInTx(importCtx, tx, file)
				file.Close()
				if uploadErr != nil {
					if key != "" {
						newObjectKeys = append(newObjectKeys, key)
					}
					return fmt.Errorf("导入封面 %s 失败：%w", item.post.CoverPath, uploadErr)
				}
				id = image.ID
				newObjectKeys = append(newObjectKeys, key)
				coverIDs[item.post.CoverPath] = id
			}
			item.post.Input.CoverMediaID = &id
		}
		_, imported, err := postStore.ImportInTx(importCtx, tx, item.post.Input, !item.post.Draft)
		if err != nil {
			return fmt.Errorf("导入文章 %s 失败：%w", item.post.Input.Slug, err)
		}
		if imported {
			created++
			messages = append(messages, fmt.Sprintf("已导入 %s", item.post.Input.Slug))
		} else {
			return fmt.Errorf("导入文章 %s 时出现并发冲突，请重试", item.post.Input.Slug)
		}
	}
	if err := tx.Commit(importCtx); err != nil {
		return fmt.Errorf("提交文章导入事务失败：%w", err)
	}
	committed = true
	for _, message := range messages {
		if _, err := fmt.Fprintln(output, message); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(output, "导入结束：新增 %d 篇，跳过 %d 篇。\n", created, skipped)
	return err
}

// markdownFiles 递归查找目录中的 Markdown 文件并按路径排序。
// 参数：root 是待扫描的文章目录。
// 返回：[]string 是排序后的文件路径；error 表示目录遍历失败。
func markdownFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

// resolveCover 将旧文章封面引用解析为资源目录中的本地路径。
// 参数：assetRoot 是旧站点资源根目录；cover 是 frontmatter 中的封面地址。
// 返回：string 是本地封面路径，外部地址或空地址返回空字符串；error 表示路径不安全。
func resolveCover(assetRoot, cover string) (string, error) {
	if cover == "" || strings.HasPrefix(cover, "http://") || strings.HasPrefix(cover, "https://") {
		return "", nil
	}
	relative := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(cover, "/")))
	if relative == "." || !filepath.IsLocal(relative) {
		return "", errors.New("封面路径不安全")
	}
	return filepath.Join(assetRoot, relative), nil
}
