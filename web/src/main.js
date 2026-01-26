function debounce(fn, delay) {
  let timeout
  return (...args) => {
    clearTimeout(timeout)
    timeout = setTimeout(() => fn(...args), delay)
  }
}

const state = {
  runs: [],
  filteredRuns: [],
  selectedRunId: null,
  selectedRun: null,
  selectedIndex: -1,
  currentView: 'details',
  searchQuery: '',
  statusFilter: '',
  sinceFilter: '',
  programFilter: '',
  branchFilter: '',
  hasChangesFilter: false,
  limit: 20,
  focusPanel: 'runs',
  showHelp: false
}

const runsList = document.getElementById('runsList')
const contentBody = document.getElementById('contentBody')
const searchInput = document.getElementById('searchInput')
const filterBtn = document.getElementById('filterBtn')
const filterCount = document.getElementById('filterCount')
const filterPopover = document.getElementById('filterPopover')
const filterChips = document.getElementById('filterChips')
const statusFilter = document.getElementById('statusFilter')
const sinceFilter = document.getElementById('sinceFilter')
const programFilter = document.getElementById('programFilter')
const branchFilter = document.getElementById('branchFilter')
const hasChangesFilter = document.getElementById('hasChangesFilter')
const limitFilter = document.getElementById('limitFilter')
const viewTabs = document.querySelectorAll('.tab')
const helpModal = document.getElementById('helpModal')

async function fetchRuns() {
  const params = new URLSearchParams()
  if (state.statusFilter) params.set('status', state.statusFilter)
  if (state.sinceFilter) params.set('since', state.sinceFilter)
  if (state.programFilter) params.set('program', state.programFilter)
  if (state.branchFilter) params.set('branch', state.branchFilter)
  if (state.hasChangesFilter) params.set('has-changes', 'true')
  if (state.limit) params.set('limit', state.limit.toString())

  const response = await fetch(`/api/runs?${params}`)
  if (!response.ok) throw new Error('Failed to fetch runs')
  return response.json()
}

async function fetchRun(id) {
  const response = await fetch(`/api/runs/${id}`)
  if (!response.ok) throw new Error('Failed to fetch run')
  return response.json()
}

async function fetchOutput(id) {
  const response = await fetch(`/api/runs/${id}/output`)
  if (!response.ok) return null
  return response.text()
}

async function fetchVersion() {
  try {
    const response = await fetch('/api/version')
    if (!response.ok) return null
    const data = await response.json()
    return data.version
  } catch {
    return null
  }
}

function formatTimestamp(timestamp) {
  const date = new Date(timestamp)
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function formatDuration(ms) {
  if (!ms) return '-'
  const seconds = Math.floor(ms / 1000)
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  if (minutes > 0) return `${minutes}m ${remainingSeconds}s`
  return `${seconds}s`
}

function formatChanges(changes) {
  if (!changes) return ''
  const parts = []
  if (changes.add > 0) parts.push(`+${changes.add}`)
  if (changes.change > 0) parts.push(`~${changes.change}`)
  if (changes.destroy > 0) parts.push(`-${changes.destroy}`)
  return parts.join(' ') || '+0'
}

function formatSyncStatus(syncStatus) {
  switch (syncStatus) {
    case 'local':
      return '<span class="sync-icon sync-local" title="Local only (not synced to S3)">↑</span>'
    case 'remote':
      return '<span class="sync-icon sync-remote" title="Remote only (not downloaded)">↓</span>'
    case 'synced':
      return '<span class="sync-icon sync-synced" title="Synced">✓</span>'
    default:
      return ''
  }
}

function filterRuns() {
  const query = state.searchQuery.toLowerCase()
  state.filteredRuns = state.runs.filter((run) => {
    if (
      query &&
      !run.workspace.toLowerCase().includes(query) &&
      !run.user?.toLowerCase().includes(query) &&
      !run.id.toLowerCase().includes(query)
    ) {
      return false
    }
    return true
  })
}

function renderRunsList() {
  if (state.filteredRuns.length === 0) {
    runsList.innerHTML = '<div class="empty-state"><p>No runs found</p></div>'
    return
  }

  runsList.innerHTML = state.filteredRuns
    .map(
      (run, index) => `
    <div class="run-item ${run.id === state.selectedRunId ? 'selected' : ''}" data-id="${run.id}" data-index="${index}">
      <div class="run-item-header">
        <div class="run-status ${run.status}"></div>
        <div class="run-workspace">${escapeHtml(run.workspace)}</div>
        <div class="run-changes">${formatChanges(run.changes)}${formatSyncStatus(run.sync_status)}</div>
      </div>
      <div class="run-item-meta">
        <span class="run-time">${formatTimestamp(run.timestamp)}</span>
        <span class="run-user">${escapeHtml(run.user || 'unknown')}</span>
      </div>
    </div>
  `
    )
    .join('')
}

function renderDetailsView(run) {
  const gitInfo = run.git || {}
  const ciInfo = run.ci || {}

  return `
    <div class="details-view">
      <div class="detail-section">
        <div class="detail-section-title">Run Info</div>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">ID</span>
            <span class="detail-value">${escapeHtml(run.id)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Status</span>
            <span class="badge badge-${run.status === 'success' ? 'success' : 'error'}">${run.status}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Workspace</span>
            <span class="detail-value">${escapeHtml(run.workspace)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Duration</span>
            <span class="detail-value">${formatDuration(run.duration_ms)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Program</span>
            <span class="detail-value">${escapeHtml(run.program || 'terraform')}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">User</span>
            <span class="detail-value">${escapeHtml(run.user || 'unknown')}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Timestamp</span>
            <span class="detail-value">${formatTimestamp(run.timestamp)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Changes</span>
            <span class="detail-value">${formatChanges(run.changes)}</span>
          </div>
        </div>
      </div>

      ${
        gitInfo.commit
          ? `
      <div class="detail-section">
        <div class="detail-section-title">Git</div>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">Commit</span>
            <span class="detail-value">${escapeHtml(gitInfo.commit)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Branch</span>
            <span class="detail-value">${escapeHtml(gitInfo.branch || '-')}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Dirty</span>
            <span class="detail-value">${gitInfo.dirty ? 'Yes' : 'No'}</span>
          </div>
          ${
            gitInfo.message
              ? `
          <div class="detail-item">
            <span class="detail-label">Message</span>
            <span class="detail-value">${escapeHtml(gitInfo.message)}</span>
          </div>
          `
              : ''
          }
        </div>
      </div>
      `
          : ''
      }

      ${
        ciInfo.provider
          ? `
      <div class="detail-section">
        <div class="detail-section-title">CI</div>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">Provider</span>
            <span class="detail-value">${escapeHtml(ciInfo.provider)}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Actor</span>
            <span class="detail-value">${escapeHtml(ciInfo.actor || '-')}</span>
          </div>
          ${
            ciInfo.workflow
              ? `
          <div class="detail-item">
            <span class="detail-label">Workflow</span>
            <span class="detail-value">${escapeHtml(ciInfo.workflow)}</span>
          </div>
          `
              : ''
          }
        </div>
      </div>
      `
          : ''
      }

      ${
        run.resources && run.resources.length > 0
          ? `
      <div class="detail-section">
        <div class="detail-section-title">Resources (${run.resources.length})</div>
        <div style="max-height: 200px; overflow-y: auto;">
          ${run.resources
            .map(
              (r) => `
            <div style="font-family: var(--font-mono); font-size: 0.8125rem; padding: 0.25rem 0; color: var(--color-text-secondary);">
              <span class="resource-action ${r.action}" style="margin-right: 0.5rem;">${r.action === 'create' ? '+' : r.action === 'destroy' ? '-' : '~'}</span>
              ${escapeHtml(r.address)}
            </div>
          `
            )
            .join('')}
        </div>
      </div>
      `
          : ''
      }
    </div>
  `
}

function formatResourceStatus(status) {
  switch (status) {
    case 'success':
      return '<span class="status-icon success">✓</span>'
    case 'failed':
      return '<span class="status-icon error">✗</span>'
    default:
      return '<span class="status-icon pending">●</span>'
  }
}

function renderEventsView(run) {
  if (!run.resources || run.resources.length === 0) {
    return '<div class="empty-state"><p>No resource events</p></div>'
  }

  return `
    <table class="events-table">
      <thead>
        <tr>
          <th>Action</th>
          <th>Resource</th>
          <th>Duration</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        ${run.resources
          .map(
            (r) => `
          <tr>
            <td><span class="resource-action ${r.action}">${r.action}</span></td>
            <td class="resource-address">${escapeHtml(r.address)}</td>
            <td class="resource-duration">${formatDuration(r.duration_ms)}</td>
            <td class="resource-status">${formatResourceStatus(r.status)}</td>
          </tr>
        `
          )
          .join('')}
      </tbody>
    </table>
  `
}

function renderTimelineView(run) {
  if (!run.resources || run.resources.length === 0) {
    return '<div class="empty-state"><p>No resource timeline</p></div>'
  }

  const resources = run.resources.filter((r) => r.duration_ms > 0 && r.start_time)

  if (resources.length === 0) {
    return '<div class="empty-state"><p>No timing data available</p></div>'
  }

  let minStartMs = Infinity
  let maxEndMs = 0
  for (const r of resources) {
    const startMs = new Date(r.start_time).getTime()
    const endMs = startMs + r.duration_ms
    if (startMs < minStartMs) minStartMs = startMs
    if (endMs > maxEndMs) maxEndMs = endMs
  }

  const totalDuration = maxEndMs - minStartMs || 1

  return `
    <div class="timeline-view">
      <div class="gantt-header">
        <div class="gantt-header-label">Resource</div>
        <div class="gantt-header-bar">
          <span>0s</span>
          <span>${formatDuration(totalDuration)}</span>
        </div>
      </div>
      <div class="gantt-chart">
        ${resources
          .map((r) => {
            const startMs = new Date(r.start_time).getTime()
            const relativeStart = startMs - minStartMs
            const left = (relativeStart / totalDuration) * 100
            const width = Math.min(Math.max((r.duration_ms / totalDuration) * 100, 0.5), 100 - left)
            return `
            <div class="gantt-row">
              <div class="gantt-label" title="${escapeHtml(r.address)}">${escapeHtml(r.address)}</div>
              <div class="gantt-bar-container">
                <div class="gantt-bar ${r.action}" style="left: ${left}%; width: ${width}%;"></div>
              </div>
            </div>
          `
          })
          .join('')}
      </div>
    </div>
  `
}

async function renderOutputView(run) {
  const output = await fetchOutput(run.id)

  if (!output) {
    return '<div class="empty-state"><p>No output available</p></div>'
  }

  return `
    <div class="output-view">
      <pre class="output-content">${escapeHtml(output)}</pre>
    </div>
  `
}

async function renderContent() {
  if (!state.selectedRun) {
    contentBody.innerHTML = '<div class="empty-state"><p>Select a run to view details</p></div>'
    return
  }

  let html = ''
  switch (state.currentView) {
    case 'details':
      html = renderDetailsView(state.selectedRun)
      break
    case 'events':
      html = renderEventsView(state.selectedRun)
      break
    case 'timeline':
      html = renderTimelineView(state.selectedRun)
      break
    case 'output':
      html = await renderOutputView(state.selectedRun)
      break
  }

  contentBody.innerHTML = html
}

function escapeHtml(text) {
  if (!text) return ''
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

async function selectRun(id, index) {
  state.selectedRunId = id
  state.selectedIndex = index
  state.selectedRun = await fetchRun(id)
  renderRunsList()
  await renderContent()
}

function selectRunByIndex(index) {
  if (index < 0 || index >= state.filteredRuns.length) return
  const run = state.filteredRuns[index]
  selectRun(run.id, index)
}

function setView(view) {
  state.currentView = view
  viewTabs.forEach((tab) => {
    tab.classList.toggle('active', tab.dataset.view === view)
  })
  renderContent()
}

async function loadRuns() {
  runsList.innerHTML = '<div class="loading">Loading runs...</div>'
  try {
    state.runs = (await fetchRuns()) || []
  } catch {
    state.runs = []
    runsList.innerHTML = '<div class="empty-state"><p>Failed to load runs</p></div>'
    return
  }
  filterRuns()
  renderRunsList()

  if (state.filteredRuns.length > 0) {
    const currentStillExists =
      state.selectedRunId && state.filteredRuns.some((r) => r.id === state.selectedRunId)
    if (!currentStillExists) {
      selectRunByIndex(0)
    }
  } else {
    state.selectedRunId = null
    state.selectedRun = null
    state.selectedIndex = -1
    renderContent()
  }
}

function navigate(delta) {
  if (state.filteredRuns.length === 0) return

  let newIndex = state.selectedIndex + delta
  if (newIndex < 0) newIndex = 0
  if (newIndex >= state.filteredRuns.length) newIndex = state.filteredRuns.length - 1

  if (newIndex !== state.selectedIndex) {
    selectRunByIndex(newIndex)
  }
}

function toggleHelp() {
  state.showHelp = !state.showHelp
  helpModal.classList.toggle('visible', state.showHelp)
}

function toggleFilterPopover() {
  filterPopover.classList.toggle('visible')
  filterBtn.classList.toggle('active', filterPopover.classList.contains('visible'))
}

function closeFilterPopover() {
  filterPopover.classList.remove('visible')
  filterBtn.classList.remove('active')
}

function getActiveFilters() {
  const filters = []
  if (state.statusFilter)
    filters.push({
      key: 'status',
      value: state.statusFilter,
      label: `status:${state.statusFilter}`
    })
  if (state.sinceFilter)
    filters.push({ key: 'since', value: state.sinceFilter, label: `since:${state.sinceFilter}` })
  if (state.programFilter)
    filters.push({
      key: 'program',
      value: state.programFilter,
      label: `program:${state.programFilter}`
    })
  if (state.branchFilter)
    filters.push({
      key: 'branch',
      value: state.branchFilter,
      label: `branch:${state.branchFilter}`
    })
  if (state.hasChangesFilter) filters.push({ key: 'hasChanges', value: true, label: 'has-changes' })
  return filters
}

function updateFilterUI() {
  const active = getActiveFilters()

  filterCount.textContent = active.length
  filterCount.classList.toggle('visible', active.length > 0)

  filterChips.innerHTML = active
    .map(
      (f) => `
    <span class="filter-chip" data-key="${f.key}">
      ${f.label}
      <span class="filter-chip-remove">×</span>
    </span>
  `
    )
    .join('')
}

function clearFilter(key) {
  switch (key) {
    case 'status':
      state.statusFilter = ''
      statusFilter.value = ''
      break
    case 'since':
      state.sinceFilter = ''
      sinceFilter.value = ''
      break
    case 'program':
      state.programFilter = ''
      programFilter.value = ''
      break
    case 'branch':
      state.branchFilter = ''
      branchFilter.value = ''
      break
    case 'hasChanges':
      state.hasChangesFilter = false
      hasChangesFilter.checked = false
      break
  }
  updateFilterUI()
  loadRuns()
}

function handleKeyDown(e) {
  if (state.showHelp) {
    if (e.key === '?' || e.key === 'Escape') {
      toggleHelp()
      e.preventDefault()
    }
    return
  }

  const isSearchFocused = document.activeElement === searchInput
  const isSelectFocused = document.activeElement.tagName === 'SELECT'

  if (isSearchFocused) {
    if (e.key === 'Escape') {
      searchInput.blur()
      if (state.searchQuery === '') {
        state.focusPanel = 'runs'
      }
      e.preventDefault()
    }
    return
  }

  if (isSelectFocused) {
    if (e.key === 'Escape') {
      document.activeElement.blur()
      e.preventDefault()
    }
    return
  }

  switch (e.key) {
    case '/':
      searchInput.focus()
      e.preventDefault()
      break
    case 'j':
    case 'ArrowDown':
      navigate(1)
      e.preventDefault()
      break
    case 'k':
    case 'ArrowUp':
      navigate(-1)
      e.preventDefault()
      break
    case 'g':
      if (!e.shiftKey) {
        selectRunByIndex(0)
        e.preventDefault()
      }
      break
    case 'G':
      selectRunByIndex(state.filteredRuns.length - 1)
      e.preventDefault()
      break
    case 'd':
      setView('details')
      e.preventDefault()
      break
    case 'e':
      setView('events')
      e.preventDefault()
      break
    case 't':
      setView('timeline')
      e.preventDefault()
      break
    case 'o':
      setView('output')
      e.preventDefault()
      break
    case '?':
      toggleHelp()
      e.preventDefault()
      break
    case 'Escape':
      state.focusPanel = 'runs'
      break
  }
}

searchInput.addEventListener('input', (e) => {
  state.searchQuery = e.target.value
  filterRuns()
  renderRunsList()
})

searchInput.addEventListener('focus', () => {
  searchInput.placeholder = 'Search runs...'
})

searchInput.addEventListener('blur', () => {
  searchInput.placeholder = '/ to search...'
})

filterBtn.addEventListener('click', (e) => {
  e.stopPropagation()
  toggleFilterPopover()
})

filterPopover.addEventListener('click', (e) => {
  e.stopPropagation()
})

filterChips.addEventListener('click', (e) => {
  const chip = e.target.closest('.filter-chip')
  if (chip) {
    clearFilter(chip.dataset.key)
  }
})

document.addEventListener('click', (e) => {
  if (!filterBtn.contains(e.target) && !filterPopover.contains(e.target)) {
    closeFilterPopover()
  }
})

statusFilter.addEventListener('change', (e) => {
  state.statusFilter = e.target.value
  updateFilterUI()
  loadRuns()
})

sinceFilter.addEventListener('change', (e) => {
  state.sinceFilter = e.target.value
  updateFilterUI()
  loadRuns()
})

programFilter.addEventListener('change', (e) => {
  state.programFilter = e.target.value
  updateFilterUI()
  loadRuns()
})

branchFilter.addEventListener(
  'input',
  debounce((e) => {
    state.branchFilter = e.target.value
    updateFilterUI()
    loadRuns()
  }, 300)
)

hasChangesFilter.addEventListener('change', (e) => {
  state.hasChangesFilter = e.target.checked
  updateFilterUI()
  loadRuns()
})

limitFilter.addEventListener('change', (e) => {
  state.limit = parseInt(e.target.value, 10)
  loadRuns()
})

runsList.addEventListener('click', (e) => {
  const runItem = e.target.closest('.run-item')
  if (runItem) {
    const index = parseInt(runItem.dataset.index, 10)
    selectRun(runItem.dataset.id, index)
  }
})

viewTabs.forEach((tab) => {
  tab.addEventListener('click', () => {
    setView(tab.dataset.view)
  })
})

helpModal.addEventListener('click', (e) => {
  if (e.target === helpModal) {
    toggleHelp()
  }
})

document.addEventListener('keydown', handleKeyDown)

async function loadVersion() {
  const version = await fetchVersion()
  const footerVersion = document.getElementById('footerVersion')
  if (version && footerVersion) {
    footerVersion.textContent = version
  }
}

loadRuns()
loadVersion()
