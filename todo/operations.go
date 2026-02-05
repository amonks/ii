package todo

import (
	"container/heap"
	"fmt"
	"sort"
	"strings"
	"time"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// CreateOptions configures a new todo.
type CreateOptions struct {
	// Status is the todo status. Defaults to StatusOpen.
	Status Status

	// Type is the todo type (task, bug, feature, design). Defaults to TypeTask.
	Type TodoType

	// Priority is the importance level (0-4). Defaults to PriorityMedium (2) when nil.
	Priority *int

	// Description provides additional context.
	Description string

	// ImplementationModel selects the model for implementing.
	ImplementationModel string

	// CodeReviewModel selects the model for commit review.
	CodeReviewModel string

	// ProjectReviewModel selects the model for project review.
	ProjectReviewModel string

	// Dependencies is a list of dependency IDs.
	Dependencies []string
}

// Create creates a new todo with the given title.
func (s *Store) Create(title string, opts CreateOptions) (*Todo, error) {
	// Validate title
	if err := ValidateTitle(title); err != nil {
		return nil, err
	}

	// Apply defaults
	if opts.Type == "" {
		opts.Type = TypeTask
	}
	normalizedType, err := normalizeTodoTypeInput(opts.Type)
	if err != nil {
		return nil, err
	}
	opts.Type = normalizedType

	priority := opts.Priority
	if priority == nil {
		defaultPriority := PriorityMedium
		priority = &defaultPriority
	}
	// Note: Priority 0 is valid (critical), so nil indicates default.
	if err := ValidatePriority(*priority); err != nil {
		return nil, err
	}

	status := opts.Status
	if status == "" {
		status = StatusOpen
	}
	normalizedStatus, err := normalizeStatusInput(status)
	if err != nil {
		return nil, err
	}

	// Parse and validate dependencies
	deps := make([]string, 0, len(opts.Dependencies))
	for _, depID := range opts.Dependencies {
		if strings.Contains(depID, ":") {
			return nil, fmt.Errorf("invalid dependency format %q: expected '<id>'", depID)
		}
		deps = append(deps, depID)
	}

	now := time.Now()
	implementationModel := internalstrings.TrimSpace(opts.ImplementationModel)
	codeReviewModel := internalstrings.TrimSpace(opts.CodeReviewModel)
	projectReviewModel := internalstrings.TrimSpace(opts.ProjectReviewModel)
	todo := Todo{
		ID:                  GenerateID(title, now),
		Title:               title,
		Description:         opts.Description,
		Status:              normalizedStatus,
		Priority:            *priority,
		Type:                opts.Type,
		ImplementationModel: implementationModel,
		CodeReviewModel:     codeReviewModel,
		ProjectReviewModel:  projectReviewModel,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// Read existing todos
	todos, err := s.readTodosWithContext()
	if err != nil {
		return nil, err
	}

	if len(deps) > 0 {
		resolvedIDs, err := resolveTodoIDsWithTodos(deps, todos)
		if err != nil {
			return nil, err
		}
		deps = resolvedIDs
		seen := make(map[string]struct{})
		for _, depID := range deps {
			if depID == todo.ID {
				return nil, ErrSelfDependency
			}
			if _, ok := seen[depID]; ok {
				return nil, ErrDuplicateDependency
			}
			seen[depID] = struct{}{}
		}
	}

	// Add the new todo
	todos = append(todos, todo)

	// Write todos
	if err := s.writeTodos(todos); err != nil {
		return nil, err
	}

	// Add dependencies
	if len(deps) > 0 {
		existingDeps, err := s.readDependenciesWithContext()
		if err != nil {
			return nil, err
		}

		for _, depID := range deps {
			existingDeps = append(existingDeps, Dependency{
				TodoID:      todo.ID,
				DependsOnID: depID,
				CreatedAt:   now,
			})
		}

		if err := s.writeDependencies(existingDeps); err != nil {
			return nil, err
		}
	}

	return &todo, nil
}

// UpdateOptions configures fields to update on todos.
// Nil pointers mean "don't update this field".
type UpdateOptions struct {
	Title               *string
	Description         *string
	Status              *Status
	Priority            *int
	Type                *TodoType
	ImplementationModel *string
	CodeReviewModel     *string
	ProjectReviewModel  *string
	DeletedAt           *time.Time
	DeleteReason        *string
	Source              *string
	StartedAt           *time.Time
	CompletedAt         *time.Time
}

// Update updates one or more todos with the given options.
// Returns the updated todos.
func (s *Store) Update(ids []string, opts UpdateOptions) ([]Todo, error) {
	todos, resolvedIDs, err := s.readTodosAndResolveIDs(ids)
	if err != nil {
		return nil, err
	}

	// Validate options
	if opts.Title != nil {
		if err := ValidateTitle(*opts.Title); err != nil {
			return nil, err
		}
	}
	if opts.Status != nil {
		normalized, err := normalizeStatusPtr(opts.Status)
		if err != nil {
			return nil, err
		}
		opts.Status = normalized
	}
	if err := validatePriorityPtr(opts.Priority); err != nil {
		return nil, err
	}
	if opts.Type != nil {
		normalized, err := normalizeTodoTypePtr(opts.Type)
		if err != nil {
			return nil, err
		}
		opts.Type = normalized
	}

	// Build a set of IDs to update
	idSet := idSetFromIDs(resolvedIDs)

	now := time.Now()
	updated := make([]Todo, 0, len(resolvedIDs))

	for i := range todos {
		if _, ok := idSet[todos[i].ID]; !ok {
			continue
		}
		delete(idSet, todos[i].ID)

		if err := applyTodoUpdates(&todos[i], opts, now); err != nil {
			return nil, fmt.Errorf("validate todo %s: %w", todos[i].ID, err)
		}

		updated = append(updated, todos[i])
	}

	// Check for unfound IDs
	if err := missingTodoIDsError(missingTodoIDsInOrder(resolvedIDs, idSet)); err != nil {
		return nil, err
	}

	if err := s.writeTodos(todos); err != nil {
		return nil, err
	}

	return updated, nil
}

func (s *Store) updateStatus(ids []string, status Status) ([]Todo, error) {
	opts := UpdateOptions{Status: &status}
	return s.Update(ids, opts)
}

// Close closes one or more todos.
func (s *Store) Close(ids []string) ([]Todo, error) {
	return s.updateStatus(ids, StatusClosed)
}

// Finish marks one or more todos as done.
func (s *Store) Finish(ids []string) ([]Todo, error) {
	return s.updateStatus(ids, StatusDone)
}

// Reopen reopens one or more closed todos.
func (s *Store) Reopen(ids []string) ([]Todo, error) {
	return s.updateStatus(ids, StatusOpen)
}

// Start marks one or more todos as in progress.
func (s *Store) Start(ids []string) ([]Todo, error) {
	return s.updateStatus(ids, StatusInProgress)
}

// Queue marks one or more todos as queued for batch processing.
func (s *Store) Queue(ids []string) ([]Todo, error) {
	return s.updateStatus(ids, StatusQueued)
}

// Delete tombstones one or more todos with an optional reason.
func (s *Store) Delete(ids []string, reason string) ([]Todo, error) {
	status := StatusTombstone
	now := time.Now()
	opts := UpdateOptions{
		Status:    &status,
		DeletedAt: &now,
	}
	if reason != "" {
		opts.DeleteReason = &reason
	}
	return s.Update(ids, opts)
}

// Show returns the full details of one or more todos.
func (s *Store) Show(ids []string) ([]Todo, error) {
	if err := validateTodoIDs(ids); err != nil {
		return nil, err
	}
	if exactIDs, ok := exactTodoIDSet(ids); ok {
		found, err := s.readTodosByExactIDs(exactIDs)
		if err != nil {
			return nil, err
		}
		result, missing := collectTodosByIDs(ids, func(id string) (Todo, bool) {
			todo, ok := found[id]
			return todo, ok
		})
		if err := missingTodoIDsError(missing); err != nil {
			return nil, err
		}
		return result, nil
	}

	todos, resolvedIDs, err := s.readTodosAndResolveIDs(ids)
	if err != nil {
		return nil, err
	}
	todoByID := todosByIDSet(todos, resolvedIDs)
	result, missing := collectTodosByIDs(resolvedIDs, func(id string) (Todo, bool) {
		todo, ok := todoByID[id]
		return todo, ok
	})

	if err := missingTodoIDsError(missing); err != nil {
		return nil, err
	}

	return result, nil
}

// ListFilter configures which todos to return.
type ListFilter struct {
	// Status filters by exact status match.
	Status *Status

	// Priority filters by exact priority match.
	Priority *int

	// Type filters by exact type match.
	Type *TodoType

	// IDs filters to specific IDs.
	IDs []string

	// TitleSubstring filters to todos with this substring in the title.
	TitleSubstring string

	// DescriptionSubstring filters to todos with this substring in the description.
	DescriptionSubstring string

	// IncludeTombstones includes soft-deleted todos. Default is false.
	IncludeTombstones bool
}

// List returns todos matching the filter.
func (s *Store) List(filter ListFilter) ([]Todo, error) {
	listed, _, err := s.listWithTodos(filter)
	return listed, err
}

// ListWithIndex returns todos matching the filter plus a full ID index.
func (s *Store) ListWithIndex(filter ListFilter) ([]Todo, IDIndex, error) {
	listed, todos, err := s.listWithTodos(filter)
	if err != nil {
		return nil, IDIndex{}, err
	}
	return listed, NewIDIndex(todos), nil
}

func (s *Store) listWithTodos(filter ListFilter) ([]Todo, []Todo, error) {
	if filter.Status != nil {
		normalized, err := normalizeStatusPtr(filter.Status)
		if err != nil {
			return nil, nil, err
		}
		filter.Status = normalized
	}
	if filter.Type != nil {
		normalized, err := normalizeTodoTypePtr(filter.Type)
		if err != nil {
			return nil, nil, err
		}
		filter.Type = normalized
	}
	if err := validatePriorityPtr(filter.Priority); err != nil {
		return nil, nil, err
	}

	titleQuery := internalstrings.NormalizeLower(filter.TitleSubstring)
	descriptionQuery := internalstrings.NormalizeLower(filter.DescriptionSubstring)

	todos, err := s.readTodosWithContext()
	if err != nil {
		return nil, nil, err
	}

	// Build ID set if filtering by IDs
	var idSet map[string]struct{}
	if len(filter.IDs) > 0 {
		resolvedIDs, err := resolveTodoIDsWithTodos(filter.IDs, todos)
		if err != nil {
			return nil, nil, err
		}
		idSet = idSetFromIDs(resolvedIDs)
	}

	includeTombstones := filter.IncludeTombstones
	if filter.Status != nil && *filter.Status == StatusTombstone {
		includeTombstones = true
	}

	result := make([]Todo, 0, len(todos))
	for _, todo := range todos {
		// Filter tombstones unless explicitly included
		if todo.Status == StatusTombstone && !includeTombstones {
			continue
		}

		// Apply filters
		if filter.Status != nil && todo.Status != *filter.Status {
			continue
		}
		if filter.Priority != nil && todo.Priority != *filter.Priority {
			continue
		}
		if filter.Type != nil && todo.Type != *filter.Type {
			continue
		}
		if idSet != nil {
			if _, ok := idSet[todo.ID]; !ok {
				continue
			}
		}
		if !containsLower(todo.Title, titleQuery) {
			continue
		}
		if !containsLower(todo.Description, descriptionQuery) {
			continue
		}

		result = append(result, todo)
	}

	return result, todos, nil
}

func containsLower(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(internalstrings.NormalizeLower(haystack), needle)
}

func idSetFromIDs(ids []string) map[string]struct{} {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func todosByIDSet(todos []Todo, ids []string) map[string]Todo {
	if len(ids) == 0 {
		return nil
	}
	idSet := idSetFromIDs(ids)
	todoByID := make(map[string]Todo, len(idSet))
	for _, todo := range todos {
		if _, ok := idSet[todo.ID]; ok {
			todoByID[todo.ID] = todo
		}
	}
	if len(todoByID) == 0 {
		return nil
	}
	return todoByID
}

func missingTodoIDsError(missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("todos not found: %s", strings.Join(missing, ", "))
}

func missingTodoIDsInOrder(ids []string, remaining map[string]struct{}) []string {
	if len(remaining) == 0 {
		return nil
	}
	missing := make([]string, 0, len(remaining))
	seen := make(map[string]struct{}, len(remaining))
	for _, id := range ids {
		if _, ok := remaining[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		missing = append(missing, id)
	}
	return missing
}

func normalizeStatusPtr(status *Status) (*Status, error) {
	if status == nil {
		return nil, nil
	}
	normalized, err := normalizeStatusInput(*status)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func normalizeTodoTypePtr(todoType *TodoType) (*TodoType, error) {
	if todoType == nil {
		return nil, nil
	}
	normalized, err := normalizeTodoTypeInput(*todoType)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func validatePriorityPtr(priority *int) error {
	if priority == nil {
		return nil
	}
	return ValidatePriority(*priority)
}

func collectTodosByIDs(ids []string, lookup func(string) (Todo, bool)) ([]Todo, []string) {
	result := make([]Todo, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	var missing []string
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		todo, ok := lookup(id)
		if !ok {
			missing = append(missing, id)
			continue
		}
		result = append(result, todo)
	}
	return result, missing
}

func applyStatusChange(item *Todo, newStatus Status, previousStatus Status, opts UpdateOptions, now time.Time) {
	item.Status = newStatus
	if newStatus != StatusDone {
		item.StartedAt = nil
		item.CompletedAt = nil
	}
	if newStatus != StatusTombstone {
		item.DeletedAt = nil
		item.DeleteReason = ""
	}

	switch newStatus {
	case StatusClosed, StatusDone:
		item.ClosedAt = &now
		if newStatus == StatusDone {
			if previousStatus == StatusInProgress {
				item.CompletedAt = &now
			} else {
				item.CompletedAt = nil
			}
		}
	case StatusTombstone:
		item.ClosedAt = nil
		if opts.DeletedAt == nil && item.DeletedAt == nil {
			item.DeletedAt = &now
		}
	case StatusOpen, StatusInProgress, StatusProposed, StatusWaiting:
		item.ClosedAt = nil
		if newStatus == StatusInProgress && previousStatus != StatusInProgress {
			item.StartedAt = &now
			item.CompletedAt = nil
		}
	}
}

func applyTodoUpdates(item *Todo, opts UpdateOptions, now time.Time) error {
	if opts.Title != nil {
		item.Title = *opts.Title
	}
	if opts.Description != nil {
		item.Description = *opts.Description
	}
	if opts.Status != nil {
		newStatus := *opts.Status
		if newStatus != item.Status {
			applyStatusChange(item, newStatus, item.Status, opts, now)
		}
	}
	if opts.Priority != nil {
		item.Priority = *opts.Priority
	}
	if opts.Type != nil {
		item.Type = *opts.Type
	}
	if opts.ImplementationModel != nil {
		item.ImplementationModel = internalstrings.TrimSpace(*opts.ImplementationModel)
	}
	if opts.CodeReviewModel != nil {
		item.CodeReviewModel = internalstrings.TrimSpace(*opts.CodeReviewModel)
	}
	if opts.ProjectReviewModel != nil {
		item.ProjectReviewModel = internalstrings.TrimSpace(*opts.ProjectReviewModel)
	}
	if opts.DeletedAt != nil {
		item.DeletedAt = opts.DeletedAt
	}
	if opts.DeleteReason != nil {
		item.DeleteReason = *opts.DeleteReason
	}
	if opts.Source != nil {
		item.Source = internalstrings.TrimSpace(*opts.Source)
	}
	if opts.StartedAt != nil {
		item.StartedAt = opts.StartedAt
	}
	if opts.CompletedAt != nil {
		item.CompletedAt = opts.CompletedAt
	}
	item.UpdatedAt = now

	return ValidateTodo(item)
}

type readyHeap struct {
	items []Todo
}

func (h readyHeap) Len() int {
	return len(h.items)
}

func (h readyHeap) Less(i, j int) bool {
	return readyLess(h.items[j], h.items[i])
}

func (h readyHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
}

func (h *readyHeap) Push(x any) {
	h.items = append(h.items, x.(Todo))
}

func (h *readyHeap) Pop() any {
	item := h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]
	return item
}

func readyLess(left, right Todo) bool {
	if left.Priority != right.Priority {
		return left.Priority < right.Priority
	}
	if TodoTypeRank(left.Type) != TodoTypeRank(right.Type) {
		return TodoTypeRank(left.Type) < TodoTypeRank(right.Type)
	}
	return left.CreatedAt.Before(right.CreatedAt)
}

// Ready returns open todos with no unresolved blockers, sorted by priority.
func (s *Store) Ready(limit int) ([]Todo, error) {
	ready, _, err := s.readyWithTodos(limit)
	return ready, err
}

// ReadyWithIndex returns ready todos plus a full ID index.
func (s *Store) ReadyWithIndex(limit int) ([]Todo, IDIndex, error) {
	ready, todos, err := s.readyWithTodos(limit)
	if err != nil {
		return nil, IDIndex{}, err
	}
	return ready, NewIDIndex(todos), nil
}

func (s *Store) readyWithTodos(limit int) ([]Todo, []Todo, error) {
	todos, err := s.readTodosWithContext()
	if err != nil {
		return nil, nil, err
	}

	deps, err := s.readDependenciesWithContext()
	if err != nil {
		return nil, nil, err
	}

	blocked := blockedTodoIDs(todos, deps)

	// Filter to open todos with no open blockers
	var ready []Todo
	var selection readyHeap
	useLimit := limit > 0
	if useLimit {
		selection = readyHeap{items: make([]Todo, 0, limit)}
	} else {
		ready = make([]Todo, 0, len(todos))
	}
	for _, todo := range todos {
		if todo.Status != StatusOpen {
			continue
		}
		if _, isBlocked := blocked[todo.ID]; isBlocked {
			continue
		}

		if useLimit {
			if len(selection.items) < limit {
				heap.Push(&selection, todo)
				continue
			}
			if readyLess(todo, selection.items[0]) {
				selection.items[0] = todo
				heap.Fix(&selection, 0)
			}
			continue
		}
		ready = append(ready, todo)
	}

	if useLimit {
		ready = selection.items
	}

	// Sort by priority (0 = highest priority)
	sort.Slice(ready, func(i, j int) bool {
		return readyLess(ready[i], ready[j])
	})

	// Apply limit
	if limit > 0 && len(ready) > limit {
		ready = ready[:limit]
	}

	return ready, todos, nil
}

func blockedTodoIDs(todos []Todo, deps []Dependency) map[string]struct{} {
	if len(deps) == 0 {
		return nil
	}

	dependsOn := make(map[string]struct{}, len(deps))
	for _, dep := range deps {
		dependsOn[dep.DependsOnID] = struct{}{}
	}
	unresolved := make(map[string]struct{})
	for _, todo := range todos {
		if _, ok := dependsOn[todo.ID]; !ok {
			continue
		}
		if !todo.Status.IsResolved() {
			unresolved[todo.ID] = struct{}{}
		}
	}
	blocked := make(map[string]struct{}, len(deps))
	for _, dep := range deps {
		if _, ok := unresolved[dep.DependsOnID]; ok {
			blocked[dep.TodoID] = struct{}{}
		}
	}
	if len(blocked) == 0 {
		return nil
	}
	return blocked
}

// DepAdd adds a dependency between two todos.
func (s *Store) DepAdd(todoID, dependsOnID string) (*Dependency, error) {
	resolvedIDs, err := s.resolveTodoIDs([]string{todoID, dependsOnID})
	if err != nil {
		return nil, err
	}
	todoID = resolvedIDs[0]
	dependsOnID = resolvedIDs[1]

	// Check for self-dependency
	if todoID == dependsOnID {
		return nil, ErrSelfDependency
	}

	// Read existing dependencies
	deps, err := s.readDependenciesWithContext()
	if err != nil {
		return nil, err
	}

	// Check for duplicate
	for _, d := range deps {
		if d.TodoID == todoID && d.DependsOnID == dependsOnID {
			return nil, ErrDuplicateDependency
		}
	}

	// Add new dependency
	dep := Dependency{
		TodoID:      todoID,
		DependsOnID: dependsOnID,
		CreatedAt:   time.Now(),
	}
	deps = append(deps, dep)

	if err := s.writeDependencies(deps); err != nil {
		return nil, err
	}

	return &dep, nil
}

// DepTree returns the dependency tree for a todo.
func (s *Store) DepTree(id string) (*DepTreeNode, error) {
	todos, resolvedIDs, err := s.readTodosAndResolveIDs([]string{id})
	if err != nil {
		return nil, err
	}
	if len(resolvedIDs) == 0 {
		return nil, ErrTodoNotFound
	}
	id = resolvedIDs[0]

	deps, err := s.readDependenciesWithContext()
	if err != nil {
		return nil, err
	}

	// Build lookup maps
	todoMap := make(map[string]*Todo, len(todos))
	for i := range todos {
		todoMap[todos[i].ID] = &todos[i]
	}

	// Group dependencies by todo ID
	depsByTodo := make(map[string][]Dependency, len(deps))
	for _, d := range deps {
		depsByTodo[d.TodoID] = append(depsByTodo[d.TodoID], d)
	}

	// Find the root todo
	rootTodo, ok := todoMap[id]
	if !ok {
		return nil, ErrTodoNotFound
	}

	// Build tree recursively
	path := make(map[string]struct{})
	return buildDepTree(rootTodo, depsByTodo, todoMap, path), nil
}

// buildDepTree recursively builds a dependency tree node.

func buildDepTree(todo *Todo, depsByTodo map[string][]Dependency, todoMap map[string]*Todo, path map[string]struct{}) *DepTreeNode {
	if _, ok := path[todo.ID]; ok {
		// Avoid cycles
		return &DepTreeNode{Todo: todo}
	}
	path[todo.ID] = struct{}{}
	defer delete(path, todo.ID)

	node := &DepTreeNode{
		Todo:     todo,
		Children: make([]*DepTreeNode, 0, len(depsByTodo[todo.ID])),
	}

	for _, dep := range depsByTodo[todo.ID] {
		childTodo, ok := todoMap[dep.DependsOnID]
		if !ok {
			continue
		}
		childNode := buildDepTree(childTodo, depsByTodo, todoMap, path)
		node.Children = append(node.Children, childNode)
	}

	return node
}
