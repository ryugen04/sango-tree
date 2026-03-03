package dag

import (
	"fmt"
	"sort"
)

// DAG はサービス間の依存関係を有向非巡回グラフとして管理する
type DAG struct {
	nodes map[string][]string // ノード名 → 依存先リスト
}

// New は空のDAGを生成する
func New() *DAG {
	return &DAG{nodes: make(map[string][]string)}
}

// AddNode はノードを追加する（依存先なし）
func (d *DAG) AddNode(name string) {
	if _, ok := d.nodes[name]; !ok {
		d.nodes[name] = nil
	}
}

// AddEdge は依存関係を追加する（from が to に依存）
func (d *DAG) AddEdge(from, to string) {
	d.AddNode(from)
	d.AddNode(to)
	d.nodes[from] = append(d.nodes[from], to)
}

// Sort はトポロジカルソートを実行し、起動順のリストを返す
// 循環依存がある場合はエラーを返す
// Kahnのアルゴリズム（入次数ベース）を使用
func (d *DAG) Sort() ([]string, error) {
	// 逆引きマップ: 依存先 → それに依存しているノード群
	dependents := make(map[string][]string)
	// 入次数を計算（エッジ: from→to は「fromがtoに依存」＝toからfromへ向かう辺）
	inDegree := make(map[string]int)
	for name := range d.nodes {
		inDegree[name] = 0
	}
	for from, deps := range d.nodes {
		for _, to := range deps {
			// fromはtoに依存 → toが処理されたらfromの入次数が減る
			dependents[to] = append(dependents[to], from)
			inDegree[from]++
		}
	}

	// 入次数0のノードをキューに追加
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var result []string
	for len(queue) > 0 {
		// 安定ソートのためアルファベット順で選択
		minIdx := 0
		for i := 1; i < len(queue); i++ {
			if queue[i] < queue[minIdx] {
				minIdx = i
			}
		}
		node := queue[minIdx]
		queue = append(queue[:minIdx], queue[minIdx+1:]...)

		result = append(result, node)

		// このノードに依存しているノードの入次数を減らす
		for _, dep := range dependents[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// 全ノードが処理されていなければ循環あり
	if len(result) != len(d.nodes) {
		var cycleNodes []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycleNodes = append(cycleNodes, name)
			}
		}
		sort.Strings(cycleNodes)
		return nil, fmt.Errorf("循環依存を検出しました: %v", cycleNodes)
	}

	// 依存先が先に来る順（起動順）
	return result, nil
}

// Reverse はトポロジカルソートの逆順（停止順）を返す
func (d *DAG) Reverse() ([]string, error) {
	sorted, err := d.Sort()
	if err != nil {
		return nil, err
	}
	reversed := make([]string, len(sorted))
	for i, s := range sorted {
		reversed[len(sorted)-1-i] = s
	}
	return reversed, nil
}

// Resolve は指定サービスとその全依存サービスを起動順で返す
func (d *DAG) Resolve(services ...string) ([]string, error) {
	// 指定サービスの存在チェック
	for _, svc := range services {
		if _, ok := d.nodes[svc]; !ok {
			return nil, fmt.Errorf("サービス %q はDAGに存在しません", svc)
		}
	}

	// 指定サービスから到達可能な全ノードを収集
	needed := make(map[string]bool)
	var collect func(name string)
	collect = func(name string) {
		if needed[name] {
			return
		}
		needed[name] = true
		for _, dep := range d.nodes[name] {
			collect(dep)
		}
	}
	for _, svc := range services {
		collect(svc)
	}

	// ソート結果からneededに含まれるものだけフィルタ
	sorted, err := d.Sort()
	if err != nil {
		return nil, err
	}
	var result []string
	for _, name := range sorted {
		if needed[name] {
			result = append(result, name)
		}
	}
	return result, nil
}

// BuildFromServices はサービスマップからDAGを構築する
// services は map[サービス名]依存先リスト の形式
func BuildFromServices(services map[string][]string) *DAG {
	d := New()
	for name, deps := range services {
		d.AddNode(name)
		for _, dep := range deps {
			d.AddEdge(name, dep)
		}
	}
	return d
}
