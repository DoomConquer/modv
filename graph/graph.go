package graph

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"
)

var graphTemplate = `digraph {
{{- if eq .direction "horizontal" -}}
rankdir=LR;
{{ end -}}
node [shape=box];
{{ range $mod, $modId := .mods -}}
{{ $modId }} [label="{{ $mod }}"];
{{ end -}}
{{- range $modId, $depModIds := .dependencies -}}
{{- range $_, $depModId := $depModIds -}}
{{ $modId }} -> {{ $depModId }};
{{  end -}}
{{- end -}}
}
`

type ModuleGraph struct {
	Reader io.Reader

	Mods         map[string]int // 依赖包名 -> modId
	Dependencies map[int][]int  // modId -> 被依赖modId
	ModIdsMap    map[int]string // modId -> 依赖包名
}

func NewModuleGraph(r io.Reader) *ModuleGraph {
	return &ModuleGraph{
		Reader: r,

		Mods:         make(map[string]int),
		Dependencies: make(map[int][]int),
		ModIdsMap:    make(map[int]string),
	}
}

func (m *ModuleGraph) Parse() error {
	bufReader := bufio.NewReader(m.Reader)

	serialID := 1
	for {
		relationBytes, err := bufReader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		relation := bytes.Split(relationBytes, []byte(" "))
		mod, depMod := strings.TrimSpace(string(relation[0])), strings.TrimSpace(string(relation[1]))

		mod = strings.Replace(mod, "@", "\n", 1)
		depMod = strings.Replace(depMod, "@", "\n", 1)

		modId, ok := m.Mods[mod]
		if !ok {
			modId = serialID
			m.Mods[mod] = modId
			m.ModIdsMap[modId] = mod
			serialID += 1
		}

		depModId, ok := m.Mods[depMod]
		if !ok {
			depModId = serialID
			m.Mods[depMod] = depModId
			m.ModIdsMap[depModId] = depMod
			serialID += 1
		}

		m.Dependencies[modId] = append(m.Dependencies[modId], depModId)
	}
}

func (m *ModuleGraph) Render(w io.Writer, args string) error {
	template, err := template.New("graph").Parse(graphTemplate)
	if err != nil {
		return fmt.Errorf("template.Parse: %+v", err)
	}

	// 只保留args包的依赖关系
	args = strings.TrimSpace(args)
	if args != "" {
		pkg := strings.Replace(args, "@", "\n", 1)
		if _, ok := m.Mods[pkg]; ok {
			filterModuleGraph := NewModuleGraph(m.Reader)
			modId := m.Mods[pkg]
			visited := map[int]int{}
			modIds := make([]int, 0)
			modIds = append(modIds, modId)
			visited[modId] = modId
			for len(modIds) > 0 {
				tmpModIds := make([]int, 0)
				for _, mId := range modIds {
					dependenciesIds := m.Dependencies[mId]
					for _, dId := range dependenciesIds {
						if _, has := visited[dId]; !has { // 防止循环依赖
							visited[dId] = dId
							tmpModIds = append(tmpModIds, dId)
						}
					}
				}
				modIds = tmpModIds
			}
			for _, visit := range visited {
				filterModuleGraph.Mods[m.ModIdsMap[visit]] = visit
				filterModuleGraph.Dependencies[visit] = m.Dependencies[visit]
			}
			if len(visited) > 0 {
				m = filterModuleGraph
			}
		} else {
			return fmt.Errorf("package %+v not existed", args)
		}
	}

	var direction string
	if len(m.Dependencies) > 15 {
		direction = "horizontal"
	}

	if err := template.Execute(w, map[string]interface{}{
		"mods":         m.Mods,
		"dependencies": m.Dependencies,
		"direction":    direction,
	}); err != nil {
		return fmt.Errorf("template.Execute: %+v", err)
	}

	return nil
}
