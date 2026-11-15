package main

// templates.go —— 内置实验模板（go:embed），供后台一键上架示例实验。
// 模板来源：plugins/workbuddy/teacherhub/templates/（与 WorkBuddy 插件包同源）。

import (
	"embed"
	"os"
	"path/filepath"
)

//go:embed plugins/workbuddy/teacherhub/templates/*.html
var embeddedTemplates embed.FS

// templateMeta 内置模板的元信息（标题/简介/标签/作者）。
var templateMeta = []struct {
	File     string
	Title    string
	Desc     string
	Tags     []string
	Keywords []string
}{
	{
		File:     "plugins/workbuddy/teacherhub/templates/实验模板-钠与水.html",
		Title:    "钠与水反应实验工作台",
		Desc:     "浮熔游响红五现象逐帧演示；钠量/水温/浓度可调，含失败与危险对照组",
		Tags:     []string{"化学", "实验", "金属钠", "演示"},
		Keywords: []string{"钠", "水", "反应", "浮熔游响红", "置换"},
	},
	{
		File:     "plugins/workbuddy/teacherhub/templates/实验模板-铁丝燃烧.html",
		Title:    "铁丝在氧气中燃烧实验工作台",
		Desc:     "火星四射黑色固体；氧气浓度/铁丝粗细/温度可调，含空气对照组",
		Tags:     []string{"化学", "实验", "燃烧", "演示"},
		Keywords: []string{"铁丝", "氧气", "燃烧", "火星四射", "四氧化三铁"},
	},
	{
		File:     "plugins/workbuddy/teacherhub/templates/实验模板-过氧化氢.html",
		Title:    "过氧化氢分解制氧气实验工作台",
		Desc:     "催化剂对照/排水集气/木条复燃；催化剂种类与浓度可调，含无催化剂对照",
		Tags:     []string{"化学", "实验", "氧气", "演示"},
		Keywords: []string{"过氧化氢", "分解", "制氧气", "催化剂", "排水集气"},
	},
	{
		File:     "plugins/workbuddy/teacherhub/templates/实验模板-凸透镜.html",
		Title:    "凸透镜成像规律演示工作台",
		Desc:     "物距扫描、三条特殊光线作图、照相机/投影仪/放大镜三场景对照",
		Tags:     []string{"物理", "实验", "光学", "演示"},
		Keywords: []string{"凸透镜", "成像", "焦距", "物距", "像距"},
	},
}

// SeedResult 单个模板上架结果。
type SeedResult struct {
	Title  string `json:"title"`
	ID     string `json:"id,omitempty"`
	Status string `json:"status"` // created / skipped
}

// seedTemplates 从 embed FS 读模板批量上架（幂等：已有同标题产物跳过）。
func seedTemplates(st *cloudStore) []SeedResult {
	// 先收集已有标题
	st.mu.Lock()
	existing := map[string]bool{}
	for _, w := range st.data.Workbenches {
		existing[w.Title] = true
	}
	st.mu.Unlock()

	results := make([]SeedResult, 0, len(templateMeta))
	for _, t := range templateMeta {
		if existing[t.Title] {
			results = append(results, SeedResult{Title: t.Title, Status: "skipped"})
			continue
		}
		data, err := embeddedTemplates.ReadFile(t.File)
		if err != nil {
			results = append(results, SeedResult{Title: t.Title, Status: "error"})
			continue
		}
		wb := Workbench{
			ID:        "wb_" + randToken()[:12],
			Title:     t.Title,
			Desc:      t.Desc,
			Tags:      t.Tags,
			Keywords:  t.Keywords,
			Teacher:   "示例",
			Size:      int64(len(data)),
			CreatedAt: 1772342400, // 2026-03-01 固定参考时间（不触发 NEW 角标）
		}

		// 写产物文件（内容存 artifacts/，不进 struct）
		_ = os.WriteFile(filepath.Join(st.dir, "artifacts", wb.ID+".html"), data, 0o644)

		st.mu.Lock()
		st.data.Workbenches = append(st.data.Workbenches, wb)
		st.log("模板上架", t.Title)
		_ = st.save()
		st.mu.Unlock()

		results = append(results, SeedResult{Title: t.Title, ID: wb.ID, Status: "created"})
	}
	return results
}
