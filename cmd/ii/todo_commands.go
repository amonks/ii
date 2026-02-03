package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/editor"
	"github.com/amonks/incrementum/internal/listflags"
	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/todo"
	"github.com/spf13/cobra"
)

var todoCmd = &cobra.Command{
	Use:   "todo",
	Short: "Manage todos for the current repository",
}

// todo create
var todoCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new todo",
	Long: `Create a new todo.

By default, opens $EDITOR to edit a TOML representation of the todo
when running interactively and no create flags are provided. Use --no-edit
to skip the editor, or --edit to force opening the editor even when not interactive.`,
	Args: cobra.NoArgs,
	RunE: runTodoCreate,
}

var (
	todoCreateTitle               string
	todoCreateType                string
	todoCreatePriority            int
	todoCreateDescription         string
	todoCreateImplementationModel string
	todoCreateCodeReviewModel     string
	todoCreateProjectReviewModel  string
	todoCreateDeps                []string
	todoCreateEdit                bool
	todoCreateNoEdit              bool
)

// todo update
var todoUpdateCmd = &cobra.Command{
	Use:   "update <id>...",
	Short: "Update one or more todos",
	Long: `Update one or more todos.

By default, opens $EDITOR to edit a TOML representation of the todo
when running interactively and no update flags are provided (one editor session per ID).
Use --no-edit to skip the editor, or --edit to force opening the editor even when not interactive.`,
	Aliases: []string{
		"edit",
	},
	Args: cobra.MinimumNArgs(1),
	RunE: runTodoUpdate,
}

var (
	todoUpdateTitle               string
	todoUpdateDescription         string
	todoUpdateStatus              string
	todoUpdatePriority            int
	todoUpdateType                string
	todoUpdateImplementationModel string
	todoUpdateCodeReviewModel     string
	todoUpdateProjectReviewModel  string
	todoUpdateEdit                bool
	todoUpdateNoEdit              bool
)

// todo close
var todoCloseCmd = &cobra.Command{
	Use:   "close <id>...",
	Short: "Close one or more todos",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoClose,
}

// todo start
var todoStartCmd = &cobra.Command{
	Use:   "start <id>...",
	Short: "Mark one or more todos as in progress",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoStart,
}

// todo finish
var todoFinishCmd = &cobra.Command{
	Use:   "finish <id>...",
	Short: "Mark one or more todos as done",
	Aliases: []string{
		"done",
	},
	Args: cobra.MinimumNArgs(1),
	RunE: runTodoFinish,
}

// todo reopen
var todoReopenCmd = &cobra.Command{
	Use:   "reopen <id>...",
	Short: "Reopen one or more closed todos",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoReopen,
}

// todo delete
var todoDeleteCmd = &cobra.Command{
	Use:   "delete <id>...",
	Short: "Delete one or more todos",
	Aliases: []string{
		"destroy",
	},
	Args: cobra.MinimumNArgs(1),
	RunE: runTodoDelete,
}

var todoDeleteReason string

// todo show
var todoShowCmd = &cobra.Command{
	Use:   "show <id>...",
	Short: "Show detailed information about todos",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoShow,
}

var todoShowJSON bool

// todo list
var todoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List todos",
	RunE:  runTodoList,
}

var (
	todoListStatus     string
	todoListPriority   int
	todoListType       string
	todoListIDs        string
	todoListTitle      string
	todoListDesc       string
	todoListJSON       bool
	todoListAll        bool
	todoListTombstones bool
)

// todo ready
var todoReadyCmd = &cobra.Command{
	Use:   "ready",
	Short: "List todos ready to work on (no unresolved blockers)",
	RunE:  runTodoReady,
}

var (
	todoReadyLimit int
	todoReadyJSON  bool
)

// todo dep
var todoDepCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage todo dependencies",
}

// todo dep add
var todoDepAddCmd = &cobra.Command{
	Use:   "add <todo-id> <depends-on-id>",
	Short: "Add a dependency between todos",
	Args:  cobra.ExactArgs(2),
	RunE:  runTodoDepAdd,
}

// todo dep tree
var todoDepTreeCmd = &cobra.Command{
	Use:   "tree <id>",
	Short: "Show dependency tree for a todo",
	Args:  cobra.ExactArgs(1),
	RunE:  runTodoDepTree,
}

func init() {
	rootCmd.AddCommand(todoCmd)
	todoCmd.AddCommand(todoCreateCmd, todoUpdateCmd, todoStartCmd, todoCloseCmd, todoFinishCmd, todoReopenCmd,
		todoDeleteCmd, todoShowCmd, todoListCmd, todoReadyCmd, todoDepCmd)
	todoDepCmd.AddCommand(todoDepAddCmd, todoDepTreeCmd)
	addDescriptionFlagAliases(todoCreateCmd, todoUpdateCmd, todoListCmd)

	// todo create flags
	todoCreateCmd.Flags().StringVar(&todoCreateTitle, "title", "", "Todo title")
	todoCreateCmd.Flags().StringVarP(&todoCreateType, "type", "t", "task", "Todo type (task, bug, feature, design)")
	todoCreateCmd.Flags().IntVarP(&todoCreatePriority, "priority", "p", todo.PriorityMedium, "Priority (0=critical, 1=high, 2=medium, 3=low, 4=backlog)")
	todoCreateCmd.Flags().StringVarP(&todoCreateDescription, "description", "d", "", "Description (use '-' to read from stdin)")
	todoCreateCmd.Flags().StringVar(&todoCreateImplementationModel, "implementation-model", "", "LLM model for implementation")
	todoCreateCmd.Flags().StringVar(&todoCreateCodeReviewModel, "code-review-model", "", "LLM model for commit review")
	todoCreateCmd.Flags().StringVar(&todoCreateProjectReviewModel, "project-review-model", "", "LLM model for project review")
	todoCreateCmd.Flags().StringArrayVar(&todoCreateDeps, "deps", nil, "Dependencies in format <id> (e.g., abc123)")
	todoCreateCmd.Flags().BoolVarP(&todoCreateEdit, "edit", "e", false, "Open $EDITOR (default if interactive and no create flags)")
	todoCreateCmd.Flags().BoolVar(&todoCreateNoEdit, "no-edit", false, "Do not open $EDITOR")

	// todo update flags
	todoUpdateCmd.Flags().StringVar(&todoUpdateTitle, "title", "", "New title")
	todoUpdateCmd.Flags().StringVarP(&todoUpdateDescription, "description", "d", "", "New description (use '-' to read from stdin)")
	todoUpdateCmd.Flags().StringVar(&todoUpdateStatus, "status", "", "New status (open, proposed, in_progress, closed, done, tombstone)")
	todoUpdateCmd.Flags().IntVar(&todoUpdatePriority, "priority", 0, "New priority (0-4)")
	todoUpdateCmd.Flags().StringVar(&todoUpdateType, "type", "", "New type (task, bug, feature, design)")
	todoUpdateCmd.Flags().StringVar(&todoUpdateImplementationModel, "implementation-model", "", "LLM model for implementation")
	todoUpdateCmd.Flags().StringVar(&todoUpdateCodeReviewModel, "code-review-model", "", "LLM model for commit review")
	todoUpdateCmd.Flags().StringVar(&todoUpdateProjectReviewModel, "project-review-model", "", "LLM model for project review")
	todoUpdateCmd.Flags().BoolVarP(&todoUpdateEdit, "edit", "e", false, "Open $EDITOR (default if interactive)")
	todoUpdateCmd.Flags().BoolVar(&todoUpdateNoEdit, "no-edit", false, "Do not open $EDITOR")

	// todo close flags

	// todo finish flags

	// todo reopen flags

	// todo delete flags
	todoDeleteCmd.Flags().StringVar(&todoDeleteReason, "reason", "", "Reason for deletion")

	// todo show flags
	todoShowCmd.Flags().BoolVar(&todoShowJSON, "json", false, "Output as JSON")

	// todo list flags
	todoListCmd.Flags().StringVar(&todoListStatus, "status", "", "Filter by status")
	todoListCmd.Flags().IntVar(&todoListPriority, "priority", -1, "Filter by priority (0-4)")
	todoListCmd.Flags().StringVar(&todoListType, "type", "", "Filter by type")
	todoListCmd.Flags().StringVar(&todoListIDs, "id", "", "Filter by IDs (comma-separated)")
	todoListCmd.Flags().StringVar(&todoListTitle, "title", "", "Filter by title substring")
	todoListCmd.Flags().StringVarP(&todoListDesc, "description", "d", "", "Filter by description substring")
	todoListCmd.Flags().BoolVar(&todoListJSON, "json", false, "Output as JSON")
	todoListCmd.Flags().BoolVar(&todoListTombstones, "tombstones", false, "Include tombstoned todos")
	listflags.AddAllFlag(todoListCmd, &todoListAll)

	// todo ready flags
	todoReadyCmd.Flags().IntVar(&todoReadyLimit, "limit", 20, "Maximum number of todos to show")
	todoReadyCmd.Flags().BoolVar(&todoReadyJSON, "json", false, "Output as JSON")

}

func todoCreatePriorityValue(cmd *cobra.Command) *int {
	if cmd.Flags().Changed("priority") {
		return todo.PriorityPtr(todoCreatePriority)
	}
	return nil
}

func runTodoCreate(cmd *cobra.Command, args []string) error {
	if err := resolveDescriptionFlag(cmd, &todoCreateDescription, os.Stdin); err != nil {
		return err
	}

	// Determine whether to open editor:
	// - --edit forces editor
	// - --no-edit skips editor
	// - otherwise, open editor only when no create fields and interactive
	hasCreateFlags := hasTodoCreateFlags(cmd)
	useEditor := shouldUseEditor(hasCreateFlags, todoCreateEdit, todoCreateNoEdit, editor.IsInteractive())

	if useEditor {
		// Pre-populate from flags if provided
		data := editor.DefaultCreateData()
		data.Status = string(defaultTodoStatus())
		if cmd.Flags().Changed("title") {
			data.Title = todoCreateTitle
		}
		if cmd.Flags().Changed("type") {
			data.Type = todoCreateType
		}
		if cmd.Flags().Changed("priority") {
			data.Priority = todoCreatePriority
		}
		if cmd.Flags().Changed("description") {
			data.Description = todoCreateDescription
		}
		if cmd.Flags().Changed("implementation-model") {
			data.ImplementationModel = todoCreateImplementationModel
		}
		if cmd.Flags().Changed("code-review-model") {
			data.CodeReviewModel = todoCreateCodeReviewModel
		}
		if cmd.Flags().Changed("project-review-model") {
			data.ProjectReviewModel = todoCreateProjectReviewModel
		}

		parsed, err := editor.EditTodoWithData(data)
		if err != nil {
			return err
		}

		store, err := openTodoStore(cmd, args)
		if err != nil {
			return err
		}
		defer store.Release()

		opts := parsed.ToCreateOptions()
		opts.Dependencies = todoCreateDeps

		created, err := store.Create(parsed.Title, opts)
		if err != nil {
			return err
		}

		highlight, err := todoLogHighlighterForStore(store)
		if err != nil {
			return err
		}
		fmt.Printf("Created todo %s: %s\n", highlight(created.ID), created.Title)
		return nil
	}

	// Non-editor path: title is required
	if todoCreateTitle == "" {
		return fmt.Errorf("title is required (use --edit to open editor)")
	}

	store, err := openTodoStore(cmd, args)
	if err != nil {
		return err
	}
	defer store.Release()

	created, err := store.Create(todoCreateTitle, todo.CreateOptions{
		Status:              defaultTodoStatus(),
		Type:                todo.TodoType(todoCreateType),
		Priority:            todoCreatePriorityValue(cmd),
		Description:         todoCreateDescription,
		ImplementationModel: todoCreateImplementationModel,
		CodeReviewModel:     todoCreateCodeReviewModel,
		ProjectReviewModel:  todoCreateProjectReviewModel,
		Dependencies:        todoCreateDeps,
	})
	if err != nil {
		return err
	}

	highlight, err := todoLogHighlighterForStore(store)
	if err != nil {
		return err
	}
	fmt.Printf("Created todo %s: %s\n", highlight(created.ID), created.Title)
	return nil
}

func runTodoUpdate(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore(cmd, args)
	if err != nil {
		return err
	}
	defer store.Release()

	if err := resolveDescriptionFlag(cmd, &todoUpdateDescription, os.Stdin); err != nil {
		return err
	}

	hasFlags := hasChangedFlags(cmd, "title", "description", "status", "priority", "type", "implementation-model", "code-review-model", "project-review-model")

	// Determine whether to open editor:
	// - --edit forces editor
	// - --no-edit skips editor
	// - otherwise, open editor only when no update flags and interactive
	useEditor := shouldUseEditor(hasFlags, todoUpdateEdit, todoUpdateNoEdit, editor.IsInteractive())
	if useEditor {
		updatedItems := make([]todo.Todo, 0, len(args))
		for _, id := range args {
			// Fetch the existing todo
			existing, err := store.Show([]string{id})
			if err != nil {
				return err
			}

			// Pre-populate from existing todo, then override with any flags
			data := editor.DataFromTodo(&existing[0])
			if cmd.Flags().Changed("title") {
				data.Title = todoUpdateTitle
			}
			if cmd.Flags().Changed("description") {
				data.Description = todoUpdateDescription
			}

			if cmd.Flags().Changed("status") {
				data.Status = todoUpdateStatus
			}
			if cmd.Flags().Changed("priority") {
				data.Priority = todoUpdatePriority
			}
			if cmd.Flags().Changed("type") {
				data.Type = todoUpdateType
			}
			if cmd.Flags().Changed("implementation-model") {
				data.ImplementationModel = todoUpdateImplementationModel
			}
			if cmd.Flags().Changed("code-review-model") {
				data.CodeReviewModel = todoUpdateCodeReviewModel
			}
			if cmd.Flags().Changed("project-review-model") {
				data.ProjectReviewModel = todoUpdateProjectReviewModel
			}

			parsed, err := editor.EditTodoWithData(data)
			if err != nil {
				return err
			}

			opts := parsed.ToUpdateOptions()
			updated, err := store.Update([]string{id}, opts)
			if err != nil {
				return err
			}
			updatedItems = append(updatedItems, updated[0])
		}

		return printTodoActionResults(store, "Updated", updatedItems)
	}

	// Non-editor path: at least one flag is required
	if !hasFlags {
		return fmt.Errorf("at least one update flag is required (use --edit to open editor)")
	}

	opts := todo.UpdateOptions{}

	if cmd.Flags().Changed("title") {
		opts.Title = &todoUpdateTitle
	}
	if cmd.Flags().Changed("description") {
		opts.Description = &todoUpdateDescription
	}
	if cmd.Flags().Changed("status") {
		status := todo.Status(todoUpdateStatus)
		opts.Status = &status
	}
	if cmd.Flags().Changed("priority") {
		opts.Priority = &todoUpdatePriority
	}
	if cmd.Flags().Changed("type") {
		typ := todo.TodoType(todoUpdateType)
		opts.Type = &typ
	}
	if cmd.Flags().Changed("implementation-model") {
		opts.ImplementationModel = &todoUpdateImplementationModel
	}
	if cmd.Flags().Changed("code-review-model") {
		opts.CodeReviewModel = &todoUpdateCodeReviewModel
	}
	if cmd.Flags().Changed("project-review-model") {
		opts.ProjectReviewModel = &todoUpdateProjectReviewModel
	}

	updated, err := store.Update(args, opts)
	if err != nil {
		return err
	}

	return printTodoActionResults(store, "Updated", updated)
}

func runTodoClose(cmd *cobra.Command, args []string) error {
	return runTodoAction(cmd, args, "Closed", func(store *todo.Store) ([]todo.Todo, error) {
		return store.Close(args)
	})
}

func runTodoStart(cmd *cobra.Command, args []string) error {
	return runTodoAction(cmd, args, "Started", func(store *todo.Store) ([]todo.Todo, error) {
		return store.Start(args)
	})
}

func runTodoFinish(cmd *cobra.Command, args []string) error {
	return runTodoAction(cmd, args, "Finished", func(store *todo.Store) ([]todo.Todo, error) {
		return store.Finish(args)
	})
}

func runTodoReopen(cmd *cobra.Command, args []string) error {
	return runTodoAction(cmd, args, "Reopened", func(store *todo.Store) ([]todo.Todo, error) {
		return store.Reopen(args)
	})
}

func runTodoDelete(cmd *cobra.Command, args []string) error {
	return runTodoAction(cmd, args, "Deleted", func(store *todo.Store) ([]todo.Todo, error) {
		return store.Delete(args, todoDeleteReason)
	})
}

func runTodoShow(cmd *cobra.Command, args []string) error {
	store, err := openTodoStoreReadOnly(cmd, args)
	if err != nil {
		return err
	}
	defer store.Release()

	todos, err := store.Show(args)
	if err != nil {
		return err
	}

	if todoShowJSON {
		return encodeJSONToStdout(todos)
	}

	highlight, err := todoLogHighlighterForStore(store)
	if err != nil {
		return err
	}
	for i, t := range todos {
		if i > 0 {
			fmt.Println("---")
		}
		printTodoDetail(t, highlight)
	}
	return nil
}

func runTodoList(cmd *cobra.Command, args []string) error {
	store, handled, err := openTodoStoreReadOnlyOrEmpty(cmd, args, todoListJSON, func() error {
		printTodoTable(nil, nil, time.Now())
		return nil
	})
	if err != nil {
		return err
	}
	if handled {
		return nil
	}
	defer store.Release()

	filter := todo.ListFilter{}

	if todoListStatus != "" {
		status := todo.Status(todoListStatus)
		filter.Status = &status
		if status == todo.StatusTombstone {
			filter.IncludeTombstones = true
		}
	}
	priority, err := todoListPriorityFilter(todoListPriority, cmd.Flags().Changed("priority"))
	if err != nil {
		return err
	}
	filter.Priority = priority
	if todoListType != "" {
		typ := todo.TodoType(todoListType)
		filter.Type = &typ
	}
	if todoListIDs != "" {
		filter.IDs = parseIDList(todoListIDs)
	}
	filter.TitleSubstring = todoListTitle
	filter.DescriptionSubstring = todoListDesc
	filter.IncludeTombstones = filter.IncludeTombstones || todoListTombstones

	var (
		todos []todo.Todo
		index todo.IDIndex
	)
	if todoListJSON {
		todos, err = store.List(filter)
	} else {
		todos, index, err = store.ListWithIndex(filter)
	}
	if err != nil {
		return err
	}

	baseTodos := todos
	if todoListStatus == "" && !todoListAll {
		filtered := baseTodos[:0]
		for _, item := range baseTodos {
			if item.Status != todo.StatusDone {
				filtered = append(filtered, item)
			}
		}
		todos = filtered
	}

	if todoListJSON {
		return encodeJSONToStdout(todos)
	}

	if len(todos) == 0 {
		var total int
		hasDone := false
		hasTombstones := false

		if todoListStatus != "" {
			allFilter := filter
			allFilter.Status = nil
			allFilter.IncludeTombstones = true
			allTodos, err := store.List(allFilter)
			if err != nil {
				return err
			}
			total = len(allTodos)
		} else {
			allTodos := baseTodos
			if !filter.IncludeTombstones {
				allFilter := filter
				allFilter.IncludeTombstones = true
				allTodos, err = store.List(allFilter)
				if err != nil {
					return err
				}
			}
			total = len(allTodos)
			for _, item := range allTodos {
				if item.Status == todo.StatusDone {
					hasDone = true
				}
				if item.Status == todo.StatusTombstone {
					hasTombstones = true
				}
			}
		}

		fmt.Println(todoEmptyListMessage(total, todoListStatus, todoListAll, filter.IncludeTombstones, hasDone, hasTombstones))
		return nil
	}

	printTodoTable(todos, index.PrefixLengths(), time.Now())
	return nil
}

func runTodoReady(cmd *cobra.Command, args []string) error {
	store, handled, err := openTodoStoreReadOnlyOrEmpty(cmd, args, todoReadyJSON, func() error {
		fmt.Println("No ready todos found.")
		return nil
	})
	if err != nil {
		return err
	}
	if handled {
		return nil
	}
	defer store.Release()

	var (
		todos []todo.Todo
		index todo.IDIndex
	)
	if todoReadyJSON {
		todos, err = store.Ready(todoReadyLimit)
	} else {
		todos, index, err = store.ReadyWithIndex(todoReadyLimit)
	}
	if err != nil {
		return err
	}

	if todoReadyJSON {
		return encodeJSONToStdout(todos)
	}

	if len(todos) == 0 {
		fmt.Println("No ready todos found.")
		return nil
	}

	printTodoTable(todos, index.PrefixLengths(), time.Now())
	return nil
}

func runTodoDepAdd(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore(cmd, args)
	if err != nil {
		return err
	}
	defer store.Release()

	dep, err := store.DepAdd(args[0], args[1])
	if err != nil {
		return err
	}

	highlight, err := todoLogHighlighterForStore(store)
	if err != nil {
		return err
	}
	fmt.Printf("Added dependency: %s depends on %s\n", highlight(dep.TodoID), highlight(dep.DependsOnID))
	return nil
}

func runTodoDepTree(cmd *cobra.Command, args []string) error {
	store, err := openTodoStoreReadOnly(cmd, args)
	if err != nil {
		return err
	}
	defer store.Release()

	tree, err := store.DepTree(args[0])
	if err != nil {
		return err
	}

	highlight, err := todoLogHighlighterForStore(store)
	if err != nil {
		return err
	}
	printDepTree(tree, "", true, highlight)
	return nil
}

func parseIDList(value string) []string {
	if value == "" {
		return nil
	}

	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
}

func todoIDPrefixLengthsForStore(store *todo.Store) (map[string]int, error) {
	index, err := store.IDIndex()
	if err != nil {
		return nil, err
	}
	return index.PrefixLengths(), nil
}

func todoLogHighlighterForStore(store *todo.Store) (func(string) string, error) {
	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return nil, err
	}
	return logHighlighter(prefixLengths, ui.HighlightID), nil
}

func todoListPriorityFilter(priority int, changed bool) (*int, error) {
	if !changed {
		return nil, nil
	}
	if err := todo.ValidatePriority(priority); err != nil {
		return nil, err
	}
	return &priority, nil
}

func runTodoAction(cmd *cobra.Command, args []string, verb string, action func(*todo.Store) ([]todo.Todo, error)) error {
	store, err := openTodoStore(cmd, args)
	if err != nil {
		return err
	}
	defer store.Release()

	items, err := action(store)
	if err != nil {
		return err
	}

	return printTodoActionResults(store, verb, items)
}

func printTodoActionResults(store *todo.Store, verb string, items []todo.Todo) error {
	highlight, err := todoLogHighlighterForStore(store)
	if err != nil {
		return err
	}
	for _, item := range items {
		fmt.Printf("%s %s: %s\n", verb, highlight(item.ID), item.Title)
	}
	return nil
}
