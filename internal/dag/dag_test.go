package dag

import (
	"slices"
	"testing"
)

// isBefore は sorted 内で a が b より前にあることを確認する
func isBefore(sorted []string, a, b string) bool {
	idxA, idxB := -1, -1
	for i, s := range sorted {
		if s == a {
			idxA = i
		}
		if s == b {
			idxB = i
		}
	}
	return idxA < idxB
}

func TestSort_Linear(t *testing.T) {
	// A→B→C: Aが Bに依存、BがCに依存 → 起動順: C, B, A
	d := New()
	d.AddEdge("A", "B")
	d.AddEdge("B", "C")

	sorted, err := d.Sort()
	if err != nil {
		t.Fatal(err)
	}
	if len(sorted) != 3 {
		t.Fatalf("ノード数が3であるべき: got %d", len(sorted))
	}
	if !isBefore(sorted, "C", "B") || !isBefore(sorted, "B", "A") {
		t.Errorf("起動順が不正: %v", sorted)
	}
}

func TestSort_Diamond(t *testing.T) {
	// A→B, A→C, B→D, C→D
	d := New()
	d.AddEdge("A", "B")
	d.AddEdge("A", "C")
	d.AddEdge("B", "D")
	d.AddEdge("C", "D")

	sorted, err := d.Sort()
	if err != nil {
		t.Fatal(err)
	}
	if len(sorted) != 4 {
		t.Fatalf("ノード数が4であるべき: got %d", len(sorted))
	}
	// Dが最初、Aが最後
	if !isBefore(sorted, "D", "B") || !isBefore(sorted, "D", "C") || !isBefore(sorted, "B", "A") || !isBefore(sorted, "C", "A") {
		t.Errorf("起動順が不正: %v", sorted)
	}
}

func TestSort_NoDependencies(t *testing.T) {
	d := New()
	d.AddNode("X")
	d.AddNode("Y")
	d.AddNode("Z")

	sorted, err := d.Sort()
	if err != nil {
		t.Fatal(err)
	}
	if len(sorted) != 3 {
		t.Fatalf("ノード数が3であるべき: got %d", len(sorted))
	}
	// 全ノードが含まれていること
	for _, name := range []string{"X", "Y", "Z"} {
		if !slices.Contains(sorted, name) {
			t.Errorf("%s が結果に含まれていない: %v", name, sorted)
		}
	}
}

func TestSort_CycleDetection(t *testing.T) {
	// A→B→C→A の循環
	d := New()
	d.AddEdge("A", "B")
	d.AddEdge("B", "C")
	d.AddEdge("C", "A")

	_, err := d.Sort()
	if err == nil {
		t.Fatal("循環依存でエラーが返されるべき")
	}
}

func TestResolve_SingleService(t *testing.T) {
	// postgres <- api <- frontend
	d := New()
	d.AddEdge("frontend", "api")
	d.AddEdge("api", "postgres")
	d.AddNode("redis")

	resolved, err := d.Resolve("api")
	if err != nil {
		t.Fatal(err)
	}
	// apiとpostgresのみ、redisは含まれない
	if len(resolved) != 2 {
		t.Fatalf("解決されるサービス数が2であるべき: got %d (%v)", len(resolved), resolved)
	}
	if !isBefore(resolved, "postgres", "api") {
		t.Errorf("起動順が不正: %v", resolved)
	}
	if slices.Contains(resolved, "redis") {
		t.Error("redisは含まれるべきでない")
	}
}

func TestResolve_MultipleServices(t *testing.T) {
	// postgres <- api, redis <- worker
	d := New()
	d.AddEdge("api", "postgres")
	d.AddEdge("worker", "redis")

	resolved, err := d.Resolve("api", "worker")
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 4 {
		t.Fatalf("解決されるサービス数が4であるべき: got %d (%v)", len(resolved), resolved)
	}
	if !isBefore(resolved, "postgres", "api") {
		t.Errorf("postgresがapiより前であるべき: %v", resolved)
	}
	if !isBefore(resolved, "redis", "worker") {
		t.Errorf("redisがworkerより前であるべき: %v", resolved)
	}
}

func TestReverse(t *testing.T) {
	d := New()
	d.AddEdge("A", "B")
	d.AddEdge("B", "C")

	sorted, err := d.Sort()
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := d.Reverse()
	if err != nil {
		t.Fatal(err)
	}
	if len(sorted) != len(reversed) {
		t.Fatalf("長さが一致しない: sort=%d, reverse=%d", len(sorted), len(reversed))
	}
	for i := range sorted {
		if sorted[i] != reversed[len(reversed)-1-i] {
			t.Errorf("逆順が不正: sort=%v, reverse=%v", sorted, reversed)
			break
		}
	}
}

func TestBuildFromServices(t *testing.T) {
	services := map[string][]string{
		"frontend": {"api"},
		"api":      {"postgres", "redis"},
		"postgres": nil,
		"redis":    nil,
	}
	d := BuildFromServices(services)
	sorted, err := d.Sort()
	if err != nil {
		t.Fatal(err)
	}
	if len(sorted) != 4 {
		t.Fatalf("ノード数が4であるべき: got %d", len(sorted))
	}
	if !isBefore(sorted, "postgres", "api") || !isBefore(sorted, "redis", "api") || !isBefore(sorted, "api", "frontend") {
		t.Errorf("起動順が不正: %v", sorted)
	}
}
