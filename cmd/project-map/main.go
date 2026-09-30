// Command project-map 根据文件职责清单生成仓库目录图。
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	descriptionsPath = "docs/project-files.json"
	outputPath       = "docs/project-structure.md"
)

type fileDescription struct {
	Path        string `json:"path"`
	Description string `json:"description"`
}

type treeNode struct {
	directories map[string]*treeNode
	files       []string
}

// main 从仓库根目录读取文件说明并写出结构图。
// 参数：无。
// 返回：无；失败时向标准错误输出中文原因并以非零状态退出。
func main() {
	root, err := os.Getwd()
	if err == nil {
		err = run(root)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "生成项目结构图失败：", err)
		os.Exit(1)
	}
}

// run 读取文件职责清单、扫描仓库并更新生成后的 Markdown 文件。
// 参数：root 是仓库根目录的绝对或相对路径。
// 返回：error；清单无效、文件说明缺失或读写失败时返回具体错误。
func run(root string) error {
	manifestPath := filepath.Join(root, filepath.FromSlash(descriptionsPath))
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("读取文件职责清单失败：%w", err)
	}
	var descriptions []fileDescription
	if err := json.Unmarshal(data, &descriptions); err != nil {
		return fmt.Errorf("文件职责清单不是有效的 JSON：%w", err)
	}
	content, err := generate(root, descriptions)
	if err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(outputPath))
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("写入项目结构图失败：%w", err)
	}
	fmt.Printf("项目结构图已更新：%s\n", outputPath)
	return nil
}

// generate 校验仓库文件与职责清单一一对应，并生成稳定排序的 Markdown。
// 参数：root 是仓库根目录；descriptions 是每个文件对应的用途说明。
// 返回：[]byte 是生成后的 Markdown 内容；error 表示扫描或清单校验失败。
func generate(root string, descriptions []fileDescription) ([]byte, error) {
	listed := make(map[string]string, len(descriptions))
	for _, item := range descriptions {
		path := filepath.ToSlash(filepath.Clean(item.Path))
		if path == "." || strings.HasPrefix(path, "../") || filepath.IsAbs(item.Path) {
			return nil, fmt.Errorf("文件职责清单中的路径无效：%q", item.Path)
		}
		if _, exists := listed[path]; exists {
			return nil, fmt.Errorf("文件职责清单中的路径重复：%s", path)
		}
		if strings.TrimSpace(item.Description) == "" {
			return nil, fmt.Errorf("文件职责说明不能为空：%s", path)
		}
		listed[path] = strings.TrimSpace(item.Description)
	}

	files, err := scanFiles(root)
	if err != nil {
		return nil, err
	}
	actual := make(map[string]struct{}, len(files))
	for _, path := range files {
		actual[path] = struct{}{}
		if _, exists := listed[path]; !exists {
			return nil, fmt.Errorf("新文件尚未登记职责说明：%s", path)
		}
	}
	for path := range listed {
		if _, exists := actual[path]; !exists {
			return nil, fmt.Errorf("职责清单中的文件不存在：%s", path)
		}
	}

	var output strings.Builder
	output.WriteString("<!-- 此文件由 go run ./cmd/project-map 自动生成，请修改 docs/project-files.json 后重新运行。 -->\n\n")
	output.WriteString("# 项目结构图\n\n")
	output.WriteString("以下目录树和文件职责表由仓库文件清单自动生成。新增、删除或移动文件后，先更新 `docs/project-files.json`，再运行 `go run ./cmd/project-map`。\n\n")
	output.WriteString("## 目录树\n\n```text\n")
	rootNode := &treeNode{directories: make(map[string]*treeNode)}
	for _, path := range files {
		insertFile(rootNode, path)
	}
	renderTree(&output, rootNode, "YukiBloom-backend", "")
	output.WriteString("```\n\n## 文件职责\n\n| 文件 | 用途 |\n| --- | --- |\n")
	for _, path := range files {
		description := strings.NewReplacer("|", "\\|", "\r", " ", "\n", " ").Replace(listed[path])
		fmt.Fprintf(&output, "| `%s` | %s |\n", path, description)
	}
	return []byte(output.String()), nil
}

// scanFiles 返回仓库中所有项目文件的相对路径，并跳过 Git 元数据目录。
// 参数：root 是仓库根目录。
// 返回：[]string 是斜杠分隔且已排序的文件路径；error 表示目录扫描失败。
func scanFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".superpowers") {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == ".env" || entry.Name() == ".DS_Store" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描项目文件失败：%w", err)
	}
	sort.Strings(files)
	return files, nil
}

// insertFile 将文件路径按目录层级插入结构树。
// 参数：root 是根节点；path 是相对仓库根目录的斜杠分隔文件路径。
// 返回：无；root 会被原地更新。
func insertFile(root *treeNode, path string) {
	parts := strings.Split(path, "/")
	current := root
	for _, part := range parts[:len(parts)-1] {
		if current.directories[part] == nil {
			current.directories[part] = &treeNode{directories: make(map[string]*treeNode)}
		}
		current = current.directories[part]
	}
	current.files = append(current.files, parts[len(parts)-1])
}

// renderTree 按名称顺序递归写出目录树的一个层级。
// 参数：output 是输出缓冲区；node 是当前目录；name 是当前节点名称；prefix 是树枝缩进。
// 返回：无；目录树文本会追加到 output。
func renderTree(output *strings.Builder, node *treeNode, name, prefix string) {
	fmt.Fprintf(output, "%s/\n", name)
	renderEntries(output, node, prefix)
}

// renderEntries 写出当前目录下的文件和子目录，并递归展开子目录。
// 参数：output 是输出缓冲区；node 是当前目录；prefix 是树枝缩进。
// 返回：无；目录项文本会追加到 output。
func renderEntries(output *strings.Builder, node *treeNode, prefix string) {
	entries := make([]string, 0, len(node.directories)+len(node.files))
	for directory := range node.directories {
		entries = append(entries, directory+"/")
	}
	for _, file := range node.files {
		entries = append(entries, file)
	}
	sort.Strings(entries)
	for index, entry := range entries {
		last := index == len(entries)-1
		branch, nextPrefix := "├── ", "│   "
		if last {
			branch, nextPrefix = "└── ", "    "
		}
		if strings.HasSuffix(entry, "/") {
			fmt.Fprintf(output, "%s%s%s\n", prefix, branch, entry)
			renderEntries(output, node.directories[strings.TrimSuffix(entry, "/")], prefix+nextPrefix)
			continue
		}
		fmt.Fprintf(output, "%s%s%s\n", prefix, branch, entry)
	}
}
