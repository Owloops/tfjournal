package tui

import (
	"fmt"
	"strings"
	"sync"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/Owloops/tfjournal/run"
	"github.com/Owloops/tfjournal/storage"
)

const (
	_runsTitle     = "Runs"
	_summaryTitle  = "Summary"
	_detailsTitle  = "Details"
	_timelineTitle = "Timeline"
	_eventsTitle   = "Events"
	_outputTitle   = "Output"
	_footerText    = "tfjournal · j/k:nav d/e/t/o:views /:search ?:help"
	_footerTextS3  = "tfjournal · j/k:nav s:sync d/e/t/o:views /:search ?:help"
	_searchPrefix  = "/"
	_noRunsMessage = "No runs found. Use 'tfjournal -- terraform apply' to record runs."
)

type viewMode int

const (
	viewModeDetails viewMode = iota
	viewModeEvents
	viewModeTimeline
	viewModeOutput
)

type syncStatus int

const (
	syncLocal syncStatus = iota
	syncRemote
	syncBoth
)

type focusPanel int

const (
	focusRuns focusPanel = iota
	focusContent
)

type App struct {
	store        storage.Store
	hybrid       *storage.HybridStore
	listOpts     storage.ListOptions
	runs         []*run.Run
	filteredRuns []*run.Run
	syncStatus   map[string]syncStatus
	selectedIdx  int
	searchMode   bool
	searchQuery  string
	showHelp     bool
	focus        focusPanel

	grid          *ui.Grid
	runsList      *widgets.List
	detailsView   *widgets.Paragraph
	eventsTable   *widgets.Table
	ganttChart    *GanttChart
	outputView    *widgets.List
	footerWidget  *widgets.Paragraph
	searchWidget  *widgets.Paragraph
	filtersWidget *widgets.Paragraph
	tabsWidget    *widgets.Paragraph

	viewMode     viewMode
	outputLines  []string
	outputScroll int
	termWidth    int
	termHeight   int

	mu        sync.Mutex
	isLoading bool
	isOffline bool
}

func New(store storage.Store, opts storage.ListOptions) *App {
	a := &App{
		store:       store,
		listOpts:    opts,
		selectedIdx: 0,
		searchMode:  false,
		showHelp:    true,
		viewMode:    viewModeDetails,
		syncStatus:  make(map[string]syncStatus),
	}
	if h, ok := store.(*storage.HybridStore); ok {
		a.hybrid = h
	}
	return a
}

func (a *App) Run() error {
	if err := ui.Init(); err != nil {
		return fmt.Errorf("failed to initialize termui: %w", err)
	}
	defer ui.Close()

	if err := a.loadRuns(); err != nil {
		return err
	}

	a.termWidth, a.termHeight = ui.TerminalDimensions()
	a.initWidgets()
	a.setupGrid()
	a.updateRunsList()
	a.updateDetails()

	ui.Render(a.grid)

	return a.eventLoop()
}

func (a *App) loadRuns() error {
	if a.hybrid != nil {
		return a.loadRunsHybrid()
	}
	runs, err := a.store.ListRuns(a.listOpts)
	if err != nil {
		return fmt.Errorf("failed to load runs: %w", err)
	}
	a.runs = runs
	a.filteredRuns = runs
	return nil
}

func (a *App) loadRunsHybrid() error {
	localRuns, err := a.hybrid.ListLocalRuns(a.listOpts)
	if err != nil {
		return fmt.Errorf("failed to load runs: %w", err)
	}

	a.mu.Lock()
	a.runs = localRuns
	a.filteredRuns = localRuns
	a.isLoading = true
	for _, r := range localRuns {
		a.syncStatus[r.ID] = syncLocal
	}
	a.mu.Unlock()

	go a.fetchS3Runs()
	return nil
}

func (a *App) fetchS3Runs() {
	localRuns, _ := a.hybrid.ListLocalRuns(a.listOpts)
	localSet := make(map[string]bool)
	for _, r := range localRuns {
		localSet[r.ID] = true
	}

	s3Runs, s3Err := a.hybrid.ListS3Runs(a.listOpts)
	s3Set := make(map[string]bool)
	if s3Err == nil {
		for _, r := range s3Runs {
			s3Set[r.ID] = true
		}
	}

	allRuns, err := a.store.ListRuns(a.listOpts)

	a.mu.Lock()
	a.isLoading = false
	a.isOffline = s3Err != nil
	if err == nil {
		a.runs = allRuns
		a.filteredRuns = filterRuns(allRuns, a.searchQuery)

		for _, r := range allRuns {
			inLocal := localSet[r.ID]
			inS3 := s3Set[r.ID]
			switch {
			case inLocal && inS3:
				a.syncStatus[r.ID] = syncBoth
			case inLocal:
				a.syncStatus[r.ID] = syncLocal
			case inS3:
				a.syncStatus[r.ID] = syncRemote
			}
		}
	}
	a.mu.Unlock()

	a.updateRunsList()
	a.updateDetails()
	ui.Render(a.grid)
}

func (a *App) initWidgets() {
	a.runsList = widgets.NewList()
	a.runsList.Title = _runsTitle
	a.runsList.BorderStyle.Fg = ui.ColorCyan
	a.runsList.TitleStyle.Fg = ui.ColorWhite
	a.runsList.TitleStyle.Modifier = ui.ModifierBold
	a.runsList.SelectedRowStyle = ui.NewStyle(ui.ColorBlack, ui.ColorCyan, ui.ModifierBold)
	a.runsList.WrapText = false

	a.detailsView = widgets.NewParagraph()
	a.detailsView.BorderStyle.Fg = ui.ColorBlue
	a.detailsView.TitleStyle.Fg = ui.ColorWhite

	a.eventsTable = widgets.NewTable()
	a.eventsTable.BorderStyle.Fg = ui.ColorBlue
	a.eventsTable.TitleStyle.Fg = ui.ColorWhite
	a.eventsTable.TextStyle = ui.NewStyle(ui.ColorWhite)
	a.eventsTable.RowSeparator = false

	a.ganttChart = NewGanttChart()
	a.ganttChart.BorderStyle.Fg = ui.ColorBlue
	a.ganttChart.TitleStyle.Fg = ui.ColorWhite

	a.outputView = widgets.NewList()
	a.outputView.BorderStyle.Fg = ui.ColorBlue
	a.outputView.TitleStyle.Fg = ui.ColorWhite
	a.outputView.WrapText = false

	a.searchWidget = widgets.NewParagraph()
	a.searchWidget.Border = true
	a.searchWidget.BorderStyle.Fg = ui.ColorCyan
	a.searchWidget.Text = "/ to search..."
	a.searchWidget.TextStyle.Fg = ui.ColorWhite

	a.filtersWidget = widgets.NewParagraph()
	a.filtersWidget.Border = true
	a.filtersWidget.BorderStyle.Fg = ui.ColorWhite
	a.filtersWidget.TextStyle.Fg = ui.ColorWhite
	a.updateFiltersText()

	a.tabsWidget = widgets.NewParagraph()
	a.tabsWidget.Border = true
	a.tabsWidget.BorderStyle.Fg = ui.ColorCyan
	a.tabsWidget.TextStyle.Fg = ui.ColorWhite
	a.updateTabsText()

	a.footerWidget = widgets.NewParagraph()
	a.footerWidget.Border = true
	a.footerWidget.BorderStyle.Fg = ui.ColorWhite
	a.footerWidget.TextStyle.Fg = ui.ColorWhite
	if a.hybrid != nil {
		a.footerWidget.Text = _footerTextS3
	} else {
		a.footerWidget.Text = _footerText
	}
}

func (a *App) setupGrid() {
	a.grid = ui.NewGrid()
	a.grid.SetRect(0, 0, a.termWidth, a.termHeight)

	a.updateTabsText()
	a.updateFiltersText()

	var contentWidget ui.Drawable
	switch a.viewMode {
	case viewModeDetails:
		contentWidget = a.detailsView
	case viewModeEvents:
		contentWidget = a.eventsTable
	case viewModeTimeline:
		contentWidget = a.ganttChart
	case viewModeOutput:
		contentWidget = a.outputView
	}

	const widgetRows = 3
	termH := float64(a.termHeight)

	searchProp := float64(widgetRows) / termH
	filtersProp := float64(widgetRows) / termH
	tabsProp := float64(widgetRows) / termH
	footerProp := float64(widgetRows) / termH

	if a.showHelp {

		mainProp := 1.0 - footerProp

		leftSearchProp := searchProp / mainProp
		leftFiltersProp := filtersProp / mainProp
		leftRunsProp := 1.0 - leftSearchProp - leftFiltersProp

		rightTabsProp := tabsProp / mainProp
		rightContentProp := 1.0 - rightTabsProp

		a.grid.Set(
			ui.NewRow(mainProp,
				ui.NewCol(0.3,
					ui.NewRow(leftSearchProp, a.searchWidget),
					ui.NewRow(leftFiltersProp, a.filtersWidget),
					ui.NewRow(leftRunsProp, a.runsList),
				),
				ui.NewCol(0.7,
					ui.NewRow(rightTabsProp, a.tabsWidget),
					ui.NewRow(rightContentProp, contentWidget),
				),
			),
			ui.NewRow(footerProp, ui.NewCol(1.0, a.footerWidget)),
		)
	} else {

		runsProp := 1.0 - searchProp - filtersProp
		contentProp := 1.0 - tabsProp

		a.grid.Set(
			ui.NewRow(1.0,
				ui.NewCol(0.3,
					ui.NewRow(searchProp, a.searchWidget),
					ui.NewRow(filtersProp, a.filtersWidget),
					ui.NewRow(runsProp, a.runsList),
				),
				ui.NewCol(0.7,
					ui.NewRow(tabsProp, a.tabsWidget),
					ui.NewRow(contentProp, contentWidget),
				),
			),
		)
	}
}

func (a *App) updateTabsText() {
	if a.tabsWidget == nil {
		return
	}

	tabs := []struct {
		mode  viewMode
		key   string
		label string
	}{
		{viewModeDetails, "d", "Details"},
		{viewModeEvents, "e", "Events"},
		{viewModeTimeline, "t", "Timeline"},
		{viewModeOutput, "o", "Output"},
	}

	var parts []string
	for _, tab := range tabs {
		if a.viewMode == tab.mode {
			parts = append(parts, fmt.Sprintf("[▶ %s:%s](fg:cyan)", tab.key, tab.label))
		} else {
			parts = append(parts, fmt.Sprintf("  %s:%s", tab.key, tab.label))
		}
	}

	a.tabsWidget.Text = strings.Join(parts, " │ ")
}

func (a *App) updateFiltersText() {
	if a.filtersWidget == nil {
		return
	}

	var parts []string

	if a.listOpts.Status != "" {
		parts = append(parts, fmt.Sprintf("status:%s", a.listOpts.Status))
	}

	if !a.listOpts.Since.IsZero() {
		parts = append(parts, "since:custom")
	}

	if a.listOpts.Program != "" {
		parts = append(parts, fmt.Sprintf("program:%s", a.listOpts.Program))
	}

	if a.listOpts.Branch != "" {
		parts = append(parts, fmt.Sprintf("branch:%s", a.listOpts.Branch))
	}

	if a.listOpts.HasChanges {
		parts = append(parts, "has-changes")
	}

	if a.listOpts.Limit > 0 {
		parts = append(parts, fmt.Sprintf("limit:%d", a.listOpts.Limit))
	}

	if len(parts) == 0 {
		a.filtersWidget.Text = "no filters"
	} else {
		a.filtersWidget.Text = strings.Join(parts, " │ ")
	}
}

func (a *App) updateRunsList() {
	a.mu.Lock()
	runs := a.filteredRuns
	totalRuns := len(a.runs)
	isLoading := a.isLoading
	isOffline := a.isOffline
	status := a.syncStatus
	a.mu.Unlock()

	if len(runs) == 0 {
		a.runsList.Rows = []string{_noRunsMessage}
		a.runsList.Title = fmt.Sprintf("%s (0)", _runsTitle)
		return
	}

	rows := make([]string, len(runs))
	for i, r := range runs {
		icon := "✓"
		iconColor := "green"
		switch r.Status {
		case run.StatusFailed:
			icon = "✗"
			iconColor = "red"
		case run.StatusRunning:
			icon = "●"
			iconColor = "yellow"
		}

		timestamp := r.Timestamp.Format("01-02 15:04")
		changes := r.ChangeSummary()

		syncIcon := ""
		if a.hybrid != nil {
			switch status[r.ID] {
			case syncLocal:
				syncIcon = " [↓](fg:yellow)"
			case syncRemote:
				syncIcon = " [↑](fg:blue)"
			case syncBoth:
				syncIcon = " [✓](fg:cyan)"
			}
		}

		rows[i] = fmt.Sprintf("[%s](fg:%s) %s %s %s%s",
			icon, iconColor, timestamp, changes, truncate(r.Workspace, 16), syncIcon)
	}

	a.runsList.Rows = rows

	var title string
	if len(runs) == totalRuns {
		title = fmt.Sprintf("%s (%d)", _runsTitle, len(runs))
	} else {
		title = fmt.Sprintf("%s (%d/%d)", _runsTitle, len(runs), totalRuns)
	}
	if isLoading {
		title += " syncing..."
	}
	if isOffline {
		title += " [offline]"
	}
	a.runsList.Title = title

	if a.selectedIdx >= len(runs) {
		a.selectedIdx = len(runs) - 1
	}
	if a.selectedIdx < 0 {
		a.selectedIdx = 0
	}
	a.runsList.SelectedRow = a.selectedIdx
}

func (a *App) updateDetails() {
	a.mu.Lock()
	runs := a.filteredRuns
	idx := a.selectedIdx
	a.mu.Unlock()

	if len(runs) == 0 || idx >= len(runs) {
		a.detailsView.Text = "No run selected"
		return
	}

	r := runs[idx]
	a.updateContentPane(r)
}

func (a *App) updateContentPane(r *run.Run) {
	switch a.viewMode {
	case viewModeDetails:
		a.updateDetailsPane(r)

	case viewModeEvents:
		a.updateEventsTable(r)

	case viewModeTimeline:
		a.ganttChart.SetData(r)

	case viewModeOutput:
		a.updateOutputPane(r)
	}
}

func (a *App) updateDetailsPane(r *run.Run) {
	details := fmt.Sprintf(`[Run:](fg:cyan)        %s
[Workspace:](fg:cyan)  %s
[Status:](fg:cyan)     %s
[Started:](fg:cyan)    %s
[Duration:](fg:cyan)   %s
[Program:](fg:cyan)    %s
[User:](fg:cyan)       %s
[Changes:](fg:cyan)    %s`,
		r.ID,
		r.Workspace,
		formatStatus(r.Status),
		r.Timestamp.Format("2006-01-02 15:04:05"),
		r.Duration().String(),
		r.Program,
		r.User,
		r.ChangeSummary(),
	)

	if r.Git != nil {
		gitLine := fmt.Sprintf("%s (%s)", r.Git.Commit, r.Git.Branch)
		if r.Git.Dirty {
			gitLine += " [dirty]"
		}
		details += fmt.Sprintf("\n[Git:](fg:cyan)        %s", gitLine)
	}

	if r.CI != nil {
		details += fmt.Sprintf("\n[CI:](fg:cyan)         %s", r.CI.Provider)
		if r.CI.Workflow != "" {
			details += fmt.Sprintf("\n[Workflow:](fg:cyan)   %s", r.CI.Workflow)
		}
	}

	if len(r.Resources) > 0 {
		details += "\n\n[Resources:](fg:yellow)"
		for _, res := range r.Resources {
			icon := "+"
			color := "green"
			switch res.Action {
			case "update":
				icon = "~"
				color = "yellow"
			case "destroy":
				icon = "-"
				color = "red"
			}
			details += fmt.Sprintf("\n  [%s](fg:%s) %s", icon, color, res.Address)
		}
	}

	a.detailsView.Text = details
}

func (a *App) updateOutputPane(r *run.Run) {
	output, err := a.store.GetOutput(r.ID)
	if err != nil {
		a.outputView.Rows = []string{fmt.Sprintf("Failed to load output: %v", err)}
		return
	}
	if len(output) == 0 {
		a.outputView.Rows = []string{"No output recorded for this run."}
		return
	}

	lines := strings.Split(string(output), "\n")
	a.outputLines = lines
	a.outputView.Rows = lines
	a.outputView.SelectedRow = 0
	a.outputScroll = 0
}

func (a *App) updateEventsTable(r *run.Run) {
	if len(r.Resources) == 0 {
		a.eventsTable.Rows = [][]string{
			{"No resource events recorded"},
		}
		return
	}

	rows := [][]string{
		{"Action", "Resource", "Duration", "Status"},
	}

	for _, res := range r.Resources {
		action := "+"
		switch res.Action {
		case "update":
			action = "~"
		case "destroy":
			action = "-"
		}

		status := "✓"
		switch res.Status {
		case "failed":
			status = "✗"
		case "running", "":
			status = "●"
		}

		duration := "-"
		if res.DurationMs > 0 {
			duration = fmt.Sprintf("%dms", res.DurationMs)
		}

		rows = append(rows, []string{action, res.Address, duration, status})
	}

	a.eventsTable.Rows = rows
	a.eventsTable.RowStyles = map[int]ui.Style{
		0: ui.NewStyle(ui.ColorCyan, ui.ColorClear, ui.ModifierBold),
	}
}

func (a *App) eventLoop() error {
	uiEvents := ui.PollEvents()

	for {
		e := <-uiEvents

		if a.searchMode {
			switch e.ID {
			case "<Escape>":
				a.exitSearch()
			case "<Backspace>", "<C-8>":
				if len(a.searchQuery) > 0 {
					a.searchQuery = a.searchQuery[:len(a.searchQuery)-1]
					a.applySearch()
				}
			default:
				if len(e.ID) == 1 {
					a.searchQuery += e.ID
					a.applySearch()
				}
			}
			continue
		}

		if a.focus == focusContent {
			switch e.ID {
			case "<Escape>":
				a.focus = focusRuns
				a.updateBorders()
				ui.Render(a.grid)
			case "j", "<Down>":
				a.scrollContent(1)
			case "k", "<Up>":
				a.scrollContent(-1)
			case "<PageDown>":
				a.scrollContentPage(1)
			case "<PageUp>":
				a.scrollContentPage(-1)
			case "g":
				a.scrollContentTop()
			case "G":
				a.scrollContentBottom()
			case "/":
				a.enterSearch()
			case "e":
				a.switchView(viewModeEvents)
			case "t":
				a.switchView(viewModeTimeline)
			case "d":
				a.switchView(viewModeDetails)
			case "o":
				a.switchView(viewModeOutput)
			case "q", "<C-c>":
				return nil
			}
			continue
		}

		switch e.ID {
		case "q", "<C-c>":
			return nil
		case "<Resize>":
			if payload, ok := e.Payload.(ui.Resize); ok {
				a.termWidth = payload.Width
				a.termHeight = payload.Height
				a.setupGrid()
				ui.Render(a.grid)
			}
		case "j", "<Down>":
			a.navigate(1)
		case "k", "<Up>":
			a.navigate(-1)
		case "g":
			a.selectedIdx = 0
			a.runsList.SelectedRow = a.selectedIdx
			a.updateDetails()
			ui.Render(a.grid)
		case "G":
			if len(a.filteredRuns) > 0 {
				a.selectedIdx = len(a.filteredRuns) - 1
				a.runsList.SelectedRow = a.selectedIdx
				a.updateDetails()
				ui.Render(a.grid)
			}
		case "<Enter>":
			a.focus = focusContent
			a.updateBorders()
			ui.Render(a.grid)
		case "e":
			a.switchView(viewModeEvents)
		case "t":
			a.switchView(viewModeTimeline)
		case "d":
			a.switchView(viewModeDetails)
		case "o":
			a.switchView(viewModeOutput)
		case "s":
			if a.hybrid != nil {
				a.syncRuns()
			}
		case "/":
			a.enterSearch()
		case "?":
			a.showHelp = !a.showHelp
			a.setupGrid()
			ui.Render(a.grid)
		}
	}
}

func (a *App) enterSearch() {
	a.searchMode = true
	a.searchQuery = ""
	a.searchWidget.Text = "type to filter..."
	a.searchWidget.BorderStyle.Fg = ui.ColorGreen
	ui.Render(a.grid)
}

func (a *App) exitSearch() {
	a.searchMode = false
	a.searchQuery = ""
	a.searchWidget.Text = "/ to search..."
	a.searchWidget.BorderStyle.Fg = ui.ColorCyan
	a.applySearch()
}

func (a *App) updateBorders() {
	if a.focus == focusContent {
		a.runsList.BorderStyle.Fg = ui.ColorWhite
		a.detailsView.BorderStyle.Fg = ui.ColorCyan
		a.eventsTable.BorderStyle.Fg = ui.ColorCyan
		a.ganttChart.BorderStyle.Fg = ui.ColorCyan
		a.outputView.BorderStyle.Fg = ui.ColorCyan
	} else {
		a.runsList.BorderStyle.Fg = ui.ColorCyan
		a.detailsView.BorderStyle.Fg = ui.ColorBlue
		a.eventsTable.BorderStyle.Fg = ui.ColorBlue
		a.ganttChart.BorderStyle.Fg = ui.ColorBlue
		a.outputView.BorderStyle.Fg = ui.ColorBlue
	}
}

func (a *App) scrollContent(delta int) {
	if a.viewMode != viewModeOutput {
		return
	}
	totalRows := len(a.outputView.Rows)
	visibleRows := a.outputView.Inner.Dy()
	if totalRows <= visibleRows {
		return
	}

	a.outputScroll = max(0, min(a.outputScroll+delta, totalRows-visibleRows))
	if delta > 0 {
		a.outputView.SelectedRow = min(a.outputScroll+visibleRows-1, totalRows-1)
	} else {
		a.outputView.SelectedRow = a.outputScroll
	}
	ui.Render(a.grid)
}

func (a *App) scrollContentPage(delta int) {
	if a.viewMode != viewModeOutput {
		return
	}
	if delta > 0 {
		a.outputView.ScrollPageDown()
	} else {
		a.outputView.ScrollPageUp()
	}
	ui.Render(a.grid)
}

func (a *App) scrollContentTop() {
	if a.viewMode != viewModeOutput {
		return
	}
	a.outputView.ScrollTop()
	ui.Render(a.grid)
}

func (a *App) scrollContentBottom() {
	if a.viewMode != viewModeOutput {
		return
	}
	a.outputView.ScrollBottom()
	ui.Render(a.grid)
}

func (a *App) syncRuns() {
	a.mu.Lock()
	if a.isLoading {
		a.mu.Unlock()
		return
	}
	a.isLoading = true
	a.mu.Unlock()

	a.updateRunsList()
	ui.Render(a.grid)

	go a.uploadAndRefresh()
}

func (a *App) uploadAndRefresh() {
	localRuns, _ := a.hybrid.ListLocalRuns(a.listOpts)
	s3Runs, _ := a.hybrid.ListS3Runs(a.listOpts)

	s3Set := make(map[string]bool)
	for _, r := range s3Runs {
		s3Set[r.ID] = true
	}

	for _, r := range localRuns {
		if !s3Set[r.ID] {
			_ = a.hybrid.UploadRun(r.ID)
		}
	}

	a.fetchS3Runs()
}

func (a *App) switchView(mode viewMode) {
	if a.viewMode == mode {
		return
	}
	a.viewMode = mode
	a.setupGrid()
	a.updateDetails()
	ui.Render(a.grid)
}

func (a *App) navigate(direction int) {
	if len(a.filteredRuns) == 0 {
		return
	}

	a.selectedIdx += direction
	if a.selectedIdx < 0 {
		a.selectedIdx = 0
	}
	if a.selectedIdx >= len(a.filteredRuns) {
		a.selectedIdx = len(a.filteredRuns) - 1
	}

	a.runsList.SelectedRow = a.selectedIdx
	a.updateDetails()
	ui.Render(a.grid)
}

func (a *App) applySearch() {
	if a.searchQuery == "" {
		a.searchWidget.Text = "type to filter..."
	} else {
		a.searchWidget.Text = _searchPrefix + a.searchQuery
	}

	var selectedID string
	if a.selectedIdx < len(a.filteredRuns) {
		selectedID = a.filteredRuns[a.selectedIdx].ID
	}

	if a.searchQuery == "" {
		a.filteredRuns = a.runs
	} else {
		a.filteredRuns = filterRuns(a.runs, a.searchQuery)
	}

	a.selectedIdx = 0
	if selectedID != "" {
		for i, r := range a.filteredRuns {
			if r.ID == selectedID {
				a.selectedIdx = i
				break
			}
		}
	}

	a.updateRunsList()
	a.updateDetails()
	ui.Render(a.grid)
}

func formatStatus(s run.Status) string {
	switch s {
	case run.StatusSuccess:
		return "[✓ success](fg:green)"
	case run.StatusFailed:
		return "[✗ failed](fg:red)"
	case run.StatusRunning:
		return "[● running](fg:yellow)"
	case run.StatusCanceled:
		return "[○ canceled](fg:white)"
	default:
		return string(s)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
