package todo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

var benchmarkNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func BenchmarkReadJSONLFromReader1K(b *testing.B) {
	benchmarkReadJSONLFromReader(b, 1000)
}

func BenchmarkReadJSONLFromReader10K(b *testing.B) {
	benchmarkReadJSONLFromReader(b, 10000)
}

func BenchmarkWriteJSONL1K(b *testing.B) {
	benchmarkWriteJSONL(b, 1000)
}

func BenchmarkWriteJSONL10K(b *testing.B) {
	benchmarkWriteJSONL(b, 10000)
}

func BenchmarkStoreList1K(b *testing.B) {
	benchmarkStoreList(b, 1000)
}

func BenchmarkStoreList10K(b *testing.B) {
	benchmarkStoreList(b, 10000)
}

func BenchmarkStoreReady1K(b *testing.B) {
	benchmarkStoreReady(b, 1000)
}

func BenchmarkStoreReady10K(b *testing.B) {
	benchmarkStoreReady(b, 10000)
}

func BenchmarkStoreReadyLimit10K(b *testing.B) {
	benchmarkStoreReadyLimit(b, 10000, 10)
}

func BenchmarkStoreShow1K(b *testing.B) {
	benchmarkStoreShow(b, 1000)
}

func BenchmarkStoreShow10K(b *testing.B) {
	benchmarkStoreShow(b, 10000)
}

func BenchmarkStoreCreate1K(b *testing.B) {
	benchmarkStoreCreate(b, 1000)
}

func BenchmarkStoreCreate10K(b *testing.B) {
	benchmarkStoreCreate(b, 10000)
}

func BenchmarkStoreDepTree1K(b *testing.B) {
	benchmarkStoreDepTree(b, 1000)
}

func BenchmarkStoreDepTree10K(b *testing.B) {
	benchmarkStoreDepTree(b, 10000)
}

func BenchmarkStoreDepAdd1K(b *testing.B) {
	benchmarkStoreDepAdd(b, 1000)
}

func BenchmarkStoreDepAdd10K(b *testing.B) {
	benchmarkStoreDepAdd(b, 10000)
}

func BenchmarkStoreUpdate1K(b *testing.B) {
	benchmarkStoreUpdate(b, 1000)
}

func BenchmarkStoreUpdate10K(b *testing.B) {
	benchmarkStoreUpdate(b, 10000)
}

func benchmarkReadJSONLFromReader(b *testing.B, count int) {
	todos := benchmarkTodos(count)

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for i := range todos {
		if err := encoder.Encode(todos[i]); err != nil {
			b.Fatalf("encode todo %d: %v", i, err)
		}
	}
	data := buffer.Bytes()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		if _, err := readJSONLFromReader[Todo](reader); err != nil {
			b.Fatalf("read todos: %v", err)
		}
	}
}

func benchmarkWriteJSONL(b *testing.B, count int) {
	todos := benchmarkTodos(count)

	path := filepath.Join(b.TempDir(), "todos.jsonl")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writeJSONL(path, todos); err != nil {
			b.Fatalf("write todos: %v", err)
		}
	}
}

func benchmarkStoreList(b *testing.B, count int) {
	store := newTestStore(b)
	todos := benchmarkTodos(count)
	if err := store.writeTodos(todos); err != nil {
		b.Fatalf("write todos: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.List(ListFilter{}); err != nil {
			b.Fatalf("list todos: %v", err)
		}
	}
}

func benchmarkStoreReady(b *testing.B, count int) {
	store := newTestStore(b)
	todos := benchmarkTodos(count)
	applyBenchmarkStatuses(todos)
	if err := store.writeTodos(todos); err != nil {
		b.Fatalf("write todos: %v", err)
	}
	deps := benchmarkDependencies(todos)
	if err := store.writeDependencies(deps); err != nil {
		b.Fatalf("write dependencies: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Ready(0); err != nil {
			b.Fatalf("ready todos: %v", err)
		}
	}
}

func benchmarkStoreReadyLimit(b *testing.B, count int, limit int) {
	store := newTestStore(b)
	todos := benchmarkTodos(count)
	applyBenchmarkStatuses(todos)
	if err := store.writeTodos(todos); err != nil {
		b.Fatalf("write todos: %v", err)
	}
	deps := benchmarkDependencies(todos)
	if err := store.writeDependencies(deps); err != nil {
		b.Fatalf("write dependencies: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Ready(limit); err != nil {
			b.Fatalf("ready todos: %v", err)
		}
	}
}

func benchmarkStoreShow(b *testing.B, count int) {
	store := newTestStore(b)
	todos := benchmarkTodos(count)
	if err := store.writeTodos(todos); err != nil {
		b.Fatalf("write todos: %v", err)
	}
	ids := []string{todos[len(todos)/2].ID}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Show(ids); err != nil {
			b.Fatalf("show todos: %v", err)
		}
	}
}

func benchmarkStoreCreate(b *testing.B, count int) {
	store := newTestStore(b)
	seed := benchmarkTodos(count)
	createTitle := "Benchmark create"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		if err := store.writeTodos(seed); err != nil {
			b.Fatalf("write todos: %v", err)
		}
		b.StartTimer()
		if _, err := store.Create(createTitle, CreateOptions{}); err != nil {
			b.Fatalf("create todo: %v", err)
		}
	}
}

func benchmarkStoreDepTree(b *testing.B, count int) {
	store := newTestStore(b)
	todos := benchmarkTodos(count)
	if err := store.writeTodos(todos); err != nil {
		b.Fatalf("write todos: %v", err)
	}
	deps := benchmarkDependencyChain(todos)
	if err := store.writeDependencies(deps); err != nil {
		b.Fatalf("write dependencies: %v", err)
	}
	rootID := todos[len(todos)-1].ID

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.DepTree(rootID); err != nil {
			b.Fatalf("dep tree: %v", err)
		}
	}
}

func benchmarkStoreDepAdd(b *testing.B, count int) {
	store := newTestStore(b)
	todos := benchmarkTodos(count)
	deps := benchmarkDependencies(todos)
	if len(todos) < 3 {
		b.Fatalf("need at least 3 todos")
	}
	todoID := todos[2].ID
	dependsOnID := todos[0].ID

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		if err := store.writeTodos(todos); err != nil {
			b.Fatalf("write todos: %v", err)
		}
		if err := store.writeDependencies(deps); err != nil {
			b.Fatalf("write dependencies: %v", err)
		}
		b.StartTimer()
		if _, err := store.DepAdd(todoID, dependsOnID); err != nil {
			b.Fatalf("dep add: %v", err)
		}
	}
}

func benchmarkStoreUpdate(b *testing.B, count int) {
	store := newTestStore(b)
	todos := benchmarkTodos(count)
	if err := store.writeTodos(todos); err != nil {
		b.Fatalf("write todos: %v", err)
	}
	ids := []string{todos[0].ID}
	updatedDescription := "Updated by benchmark"
	opts := UpdateOptions{Description: &updatedDescription}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Update(ids, opts); err != nil {
			b.Fatalf("update todos: %v", err)
		}
	}
}

func benchmarkTodos(count int) []Todo {
	todos := make([]Todo, count)
	for i := range count {
		title := fmt.Sprintf("Todo %d", i)
		todos[i] = Todo{
			ID:          GenerateID(title, benchmarkNow.Add(time.Duration(i)*time.Second)),
			Title:       title,
			Description: "Benchmark payload for todo store.",
			Status:      StatusOpen,
			Priority:    PriorityMedium,
			Type:        TypeTask,
			CreatedAt:   benchmarkNow,
			UpdatedAt:   benchmarkNow,
		}
	}
	return todos
}

func applyBenchmarkStatuses(todos []Todo) {
	for i := range todos {
		switch i % 3 {
		case 0:
			todos[i].Status = StatusOpen
		case 1:
			todos[i].Status = StatusInProgress
		case 2:
			todos[i].Status = StatusDone
		}
	}
}

func benchmarkDependencies(todos []Todo) []Dependency {
	deps := make([]Dependency, 0, len(todos)/4)
	for i := 1; i < len(todos); i += 4 {
		deps = append(deps, Dependency{
			TodoID:      todos[i].ID,
			DependsOnID: todos[i-1].ID,
			CreatedAt:   benchmarkNow,
		})
	}
	return deps
}

func benchmarkDependencyChain(todos []Todo) []Dependency {
	if len(todos) < 2 {
		return nil
	}
	deps := make([]Dependency, 0, len(todos)-1)
	for i := 1; i < len(todos); i++ {
		deps = append(deps, Dependency{
			TodoID:      todos[i].ID,
			DependsOnID: todos[i-1].ID,
			CreatedAt:   benchmarkNow,
		})
	}
	return deps
}
