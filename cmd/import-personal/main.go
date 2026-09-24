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
	"github.com/SmileSnow819/YukiBloom-backend/internal/personal"
	yaml "github.com/goccy/go-yaml"
)

type options struct {
	footprints string
	timeline   string
	assets     string
	apply      bool
	replace    bool
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv, os.Stdout); err != nil {
		log.Printf("个人内容导入未完成：%v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, output io.Writer) error {
	flags := flag.NewFlagSet("import-personal", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	settings := options{}
	flags.StringVar(&settings.footprints, "footprints", "../YukiBloom/config/footprints.yaml", "足迹 YAML 文件")
	flags.StringVar(&settings.timeline, "timeline", "../YukiBloom/config/timeline.yaml", "实习经历 YAML 文件")
	flags.StringVar(&settings.assets, "assets", "../YukiBloom/public", "Astro public 目录")
	flags.BoolVar(&settings.apply, "apply", false, "执行导入；省略时只做预检查")
	flags.BoolVar(&settings.replace, "replace", false, "已有记录时整体替换数据库内容")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(output, "用法：go run ./cmd/import-personal [-footprints 文件] [-timeline 文件] [-assets 图片目录] [-apply] [-replace]\n默认只做预检查；已有数据时需要显式添加 -replace 才会覆盖。")
			return writeErr
		}
		return errors.New("命令参数不正确，请检查导入参数")
	}
	var footprintData personal.Footprints
	if err := readYAML(settings.footprints, &footprintData); err != nil {
		return fmt.Errorf("读取足迹配置失败：%w", err)
	}
	var timelineConfig struct {
		Items []personal.Internship `yaml:"timeline"`
	}
	if err := readYAML(settings.timeline, &timelineConfig); err != nil {
		return fmt.Errorf("读取实习经历配置失败：%w", err)
	}
	if err := personal.ValidateFootprints(footprintData); err != nil {
		return fmt.Errorf("足迹配置有误：%w", err)
	}
	if err := personal.ValidateTimeline(timelineConfig.Items); err != nil {
		return fmt.Errorf("实习经历配置有误：%w", err)
	}
	for routeIndex := range footprintData.Routes {
		for imageIndex, imageURL := range footprintData.Routes[routeIndex].Images {
			file, err := resolveAsset(settings.assets, imageURL)
			if err != nil {
				return fmt.Errorf("路线 %d 图片有误：%w", routeIndex+1, err)
			}
			if file == "" {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("路线 %d 图片 %s 读取失败：%w", routeIndex+1, imageURL, err)
			}
			if _, err := media.Validate(data); err != nil {
				return fmt.Errorf("路线 %d 图片 %s 无效：%w", routeIndex+1, imageURL, err)
			}
			footprintData.Routes[routeIndex].Images[imageIndex] = imageURL
		}
	}
	if _, err := fmt.Fprintf(output, "预检查完成：%d 个地点、%d 段停留、%d 条路线（含图片）、%d 条实习经历，未发现格式问题。\n", len(footprintData.Locations), len(footprintData.Stays), len(footprintData.Routes), len(timelineConfig.Items)); err != nil {
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
	var hasData bool
	if err := pool.QueryRow(startupCtx, `SELECT
		(SELECT EXISTS(SELECT 1 FROM footprint_locations)) OR
		(SELECT EXISTS(SELECT 1 FROM internship_experiences))`).Scan(&hasData); err != nil {
		return fmt.Errorf("检查现有个人内容失败：%w", err)
	}
	if hasData && !settings.replace {
		return errors.New("数据库已有足迹或实习数据。为避免覆盖后台修改，确认替换后再添加 -replace")
	}
	mediaStore := media.NewStore(pool, cfg.UploadDir)
	newMedia := make([]string, 0)
	imageCache := make(map[string]string)
	for routeIndex := range footprintData.Routes {
		for imageIndex, imageURL := range footprintData.Routes[routeIndex].Images {
			file, err := resolveAsset(settings.assets, imageURL)
			if err != nil || file == "" {
				continue
			}
			storedURL, exists := imageCache[imageURL]
			if !exists {
				opened, err := os.Open(file)
				if err != nil {
					cleanupMedia(startupCtx, mediaStore, newMedia)
					return fmt.Errorf("打开路线图片失败：%w", err)
				}
				item, uploadErr := mediaStore.Save(startupCtx, opened)
				opened.Close()
				if uploadErr != nil {
					cleanupMedia(startupCtx, mediaStore, newMedia)
					return fmt.Errorf("导入路线图片失败：%w", uploadErr)
				}
				storedURL = item.URL
				imageCache[imageURL] = storedURL
				newMedia = append(newMedia, item.ID)
			}
			footprintData.Routes[routeIndex].Images[imageIndex] = storedURL
		}
	}
	personalStore := personal.NewStore(pool)
	if err := personalStore.ReplaceAll(startupCtx, footprintData, timelineConfig.Items); err != nil {
		cleanupMedia(startupCtx, mediaStore, newMedia)
		return fmt.Errorf("导入足迹和实习经历失败：%w", err)
	}
	_, err = fmt.Fprintln(output, "个人内容已导入：足迹和实习经历都已保存到数据库。")
	return err
}

func readYAML(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, value); err != nil {
		return err
	}
	return nil
}

func resolveAsset(root, reference string) (string, error) {
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
