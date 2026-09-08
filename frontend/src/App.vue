<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import * as XLSX from 'xlsx'
import * as echarts from 'echarts'
import chinaGeoJson from 'china-geojson/src/geojson/china.json'

type User = { id: string; username: string; name: string; role: string; status?: string }
type Project = { id?: string; province: string; name: string; source?: string; type?: string; specificType?: string; projectYear: string; responsibleDepartment?: string; expansionOwner?: string; leadParticipatingUnits?: string; totalAmount?: number; contractAmount?: number; successDate?: string; zhixinRole?: string; internalSupportDepartment?: string; provincialSupportDepartment?: string; governmentUnit?: string; stage: string; health: string; progress?: string; problemTags?: string[]; problemDescription?: string; nextPlan?: string; updateCycle: string; customCycleDays?: number; updatedAt?: string; nextUpdateAt?: string; updatedBy?: string; createdAt?: string; createdBy?: string; version?: number }
type History = { id: number; stage?: string; health?: string; problemTags?: string[]; progress?: string; problemDescription?: string; nextPlan?: string; note?: string; updatedAt: string; updatedBy: string; correctionOf?: number; correctionNote?: string }
type Detail = { project: Project; history: History[] }
type Dict = { provinces: string[]; projectTypes: string[]; stages: string[]; healthStatuses: string[]; problemTags: string[]; updateCycles: string[]; nearDays: number }

const token = ref(localStorage.getItem('ledger-token') || '')
const user = ref<User | null>(JSON.parse(localStorage.getItem('ledger-user') || 'null'))
const error = ref('')
const loading = ref(false)
const view = ref<'ledger' | 'panorama' | 'mine' | 'config'>('ledger')
const projects = ref<Project[]>([])
const total = ref(0)
const dict = ref<Dict>({ provinces: [], projectTypes: [], stages: [], healthStatuses: [], problemTags: [], updateCycles: [], nearDays: 3 })
const selected = ref<Detail | null>(null)
const showForm = ref(false)
const savingForm = ref(false)

const showUserForm = ref(false)
const showPasswordForm = ref(false)
const showAccountMenu = ref(false)
const accountMenu = ref<HTMLElement | null>(null)
const showStageMenu = ref(false)
const stageMenu = ref<HTMLElement | null>(null)
const showLedgerStats = ref(false)
const showClearProjects = ref(false)
const deleteTarget = ref<Project | null>(null)
const deletingProject = ref(false)
const resetTarget = ref<User | null>(null)
const correctionTarget = ref<History | null>(null)
const correction = reactive({ progress: '', problemDescription: '', nextPlan: '', note: '', correctionNote: '' })
const newUser = reactive({ name: '', username: '', password: '', role: 'user' })
const passwordForm = reactive({ currentPassword: '', newPassword: '', confirmPassword: '' })
const loginForm = reactive({ username: '', password: '' })
const resetPassword = ref('')
const formMode = ref<'new' | 'edit' | 'progress'>('new')
const form = reactive<Project>({ province: '', name: '', projectYear: String(new Date().getFullYear()), stage: '', health: '', updateCycle: '', problemTags: [] })
const note = ref('')
const filters = reactive({ q: '', province: '', stage: [] as string[], health: '', type: '', specificType: '', expansionOwner: '', projectYear: '', successDateFrom: '', successDateTo: '', quick: '', sort: '', page: 1, pageSize: 1000 })
const selectedProjectIds = ref<string[]>([])
const showExportColumns = ref(false)
const exportColumnKeys = ref<string[]>([])
const exportColumns = [
  ['province', '省份'], ['name', '项目名称'], ['type', '项目类型'], ['specificType', '具体类型'],
  ['projectYear', '项目年份'], ['responsibleDepartment', '主责部门'], ['expansionOwner', '拓展组负责人'],
  ['leadParticipatingUnits', '项目牵头参与单位'], ['totalAmount', '项目总金额'], ['contractAmount', '智芯合同金额'],
  ['source', '来源'], ['successDate', '申报成功时间'], ['zhixinRole', '智芯角色'],
  ['internalSupportDepartment', '智芯内部支撑部门'], ['provincialSupportDepartment', '省公司支撑部门'],
  ['governmentUnit', '政府主管单位或其他相关单位'], ['stage', '项目阶段'], ['health', '健康状态'],
  ['problemTags', '问题标签'], ['progress', '当前进展'], ['problemDescription', '问题说明'],
  ['nextPlan', '下一步计划'], ['updateCycle', '更新周期']
] as const
const exportProjectTypeOrder = ['政府项目', '省部级项目', '国网项目', '省公司科技项目']
const defaultExportColumnKeys = exportColumns.map(([key]) => key)
const ownerOptions = computed(() => users.value.filter(item => item.status !== '停用').map(item => item.name).filter(Boolean).sort((a, b) => a.localeCompare(b, 'zh-CN')))
const formStageOptions = computed(() => [...new Set([...(dict.value.stages || []), form.stage].filter(Boolean))])
const formOwnerOptions = computed(() => [...new Set([...(ownerOptions.value || []), form.expansionOwner].filter(Boolean))])
const sortKey = ref<'province' | 'name' | 'projectYear' | 'type' | 'stage' | 'health' | 'progress' | 'expansionOwner' | 'updatedAt' | 'status'>('updatedAt')
const sortDirection = ref<'asc' | 'desc'>('desc')
const resizingColumn = ref(false)
const columnWidths = reactive<Record<string, number>>({ selected: 42, province: 76, name: 280, projectYear: 72, type: 110, specificType: 150, stage: 100, health: 90, progress: 280, expansionOwner: 110, status: 110, updatedAt: 150 })
const sortedProjects = computed(() => [...projects.value].sort((a, b) => { const key = sortKey.value; const av = key === 'status' ? status(a) : String(a[key] || ''); const bv = key === 'status' ? status(b) : String(b[key] || ''); const result = av.localeCompare(bv, 'zh-CN'); return sortDirection.value === 'asc' ? result : -result }))
const statisticProjects = computed(() => {
  const source = selectedProjectIds.value.length
    ? projects.value.filter(project => project.id && selectedProjectIds.value.includes(project.id))
    : projects.value
  const seen = new Set<string>()
  return source.filter(project => {
    const name = String(project.name || '').trim()
    if (seen.has(name)) return false
    seen.add(name)
    return true
  })
})
const ledgerStats = computed(() => { const groups = new Map<string, { type: string; count: number; totalAmount: number; contractAmount: number }>(); for (const project of statisticProjects.value) { const type = project.type || '未填写'; const group = groups.get(type) || { type, count: 0, totalAmount: 0, contractAmount: 0 }; group.count += 1; group.totalAmount += Number(project.totalAmount || 0); group.contractAmount += Number(project.contractAmount || 0); groups.set(type, group) } return [...groups.values()].sort((a, b) => b.count - a.count) })
const ledgerTotalAmount = computed(() => statisticProjects.value.reduce((sum, project) => sum + Number(project.totalAmount || 0), 0))
const ledgerContractAmount = computed(() => statisticProjects.value.reduce((sum, project) => sum + Number(project.contractAmount || 0), 0))
const stats = ref<Array<{ province: string; total: number; issues: number }>>([])
const users = ref<User[]>([])
const importRows = ref<Project[]>([])
const importResult = ref<{ created: number; updated: number; errors: Array<{ row: number; message: string }>; done?: boolean } | null>(null)
const configDraft = reactive<any>({ provinces: '', projectTypes: '', stages: '', healthStatuses: '', problemTags: '', updateCycles: '', nearDays: 3 })
const directUnitNames = new Set(['中国电科院', '国网经研院', '国网能源院', '国网工研院', '国网信通中心（大数据中心）', '国网特高压公司', '国网直流中心'])
const provinceOptions = computed(() => dict.value.provinces.filter(x => !directUnitNames.has(x)))
const directUnitOptions = computed(() => dict.value.provinces.filter(x => directUnitNames.has(x)))

async function api<T>(url: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(url, { ...options, cache: 'no-store', headers: { 'Content-Type': 'application/json', ...(token.value ? { Authorization: `Bearer ${token.value}` } : {}), ...(options.headers || {}) } })
  const body = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(body.error || '请求失败')
  return (body.data ?? body) as T
}
async function loadDict() { dict.value = await api<Dict>('/api/dictionaries') }
function filterParams() { const q = new URLSearchParams({ page: '1', pageSize: String(filters.pageSize) }); Object.entries(filters).forEach(([k, v]) => { if (k === 'page' || k === 'pageSize') return; const value = Array.isArray(v) ? v.join(',') : String(v); if (value) q.set(k, value) }); return q }
async function loadProjects() { loading.value = true; try { const data = await api<{ items: Project[]; total: number }>(`/api/projects?${filterParams()}`); projects.value = data.items; dict.value.stages = [...new Set([...dict.value.stages, ...data.items.map(item => item.stage).filter(Boolean)])]; total.value = data.total } finally { loading.value = false } }
function applyFilters() { filters.page = 1; loadProjects() }
function resetFilters() { Object.assign(filters, { q: '', province: '', stage: [], health: '', type: '', specificType: '', expansionOwner: '', projectYear: '', successDateFrom: '', successDateTo: '', quick: '', sort: '', page: 1 }); selectedProjectIds.value = []; loadProjects() }
function toggleStage(stage: string) { filters.stage = filters.stage.includes(stage) ? filters.stage.filter(item => item !== stage) : [...filters.stage, stage] }
function removeStage(stage: string) { filters.stage = filters.stage.filter(item => item !== stage) }
function toggleProject(id?: string) { if (!id) return; selectedProjectIds.value = selectedProjectIds.value.includes(id) ? selectedProjectIds.value.filter(item => item !== id) : [...selectedProjectIds.value, id] }
function toggleAllProjects() { const ids = sortedProjects.value.map(project => project.id).filter(Boolean) as string[]; selectedProjectIds.value = ids.every(id => selectedProjectIds.value.includes(id)) ? selectedProjectIds.value.filter(id => !ids.includes(id)) : [...new Set([...selectedProjectIds.value, ...ids])] }
function setSort(key: typeof sortKey.value) { if (resizingColumn.value) return; if (sortKey.value === key) sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'; else { sortKey.value = key; sortDirection.value = key === 'updatedAt' ? 'desc' : 'asc' } }
function startResize(event: PointerEvent, key: string) { resizingColumn.value = true; const startX = event.clientX; const startWidth = columnWidths[key]; const move = (e: PointerEvent) => { columnWidths[key] = Math.max(60, startWidth + e.clientX - startX); if (key === 'province') document.documentElement.style.setProperty('--ledger-province-width', `${columnWidths[key]}px`) }; const stop = () => { window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', stop); window.setTimeout(() => { resizingColumn.value = false }, 0) }; window.addEventListener('pointermove', move); window.addEventListener('pointerup', stop) }
async function loadStats() { stats.value = await api('/api/projects/summary') }
async function boot() { if (!user.value) return; try { await loadDict(); Object.assign(configDraft, { ...dict.value, provinces: dict.value.provinces.join('，'), projectTypes: dict.value.projectTypes.join('，'), stages: dict.value.stages.join('，'), healthStatuses: dict.value.healthStatuses.join('，'), problemTags: dict.value.problemTags.join('，'), updateCycles: dict.value.updateCycles.join('，') }); await loadProjects(); users.value = await api<User[]>('/api/users') } catch (e) { logout(); error.value = (e as Error).message } }
async function login(username: string, password: string) { try { const data = await api<{ token: string; user: User }>('/api/login', { method: 'POST', body: JSON.stringify({ username, password }) }); token.value = data.token; user.value = data.user; localStorage.setItem('ledger-token', data.token); localStorage.setItem('ledger-user', JSON.stringify(data.user)); error.value = ''; await boot() } catch (e) { error.value = (e as Error).message } }
function logout() { showAccountMenu.value = false; selected.value = null; showForm.value = false; view.value = 'ledger'; token.value = ''; user.value = null; localStorage.removeItem('ledger-token'); localStorage.removeItem('ledger-user') }
function pushMobileOverlay() { if (window.matchMedia('(max-width: 800px)').matches) window.history.pushState({ mobileOverlay: true }, '') }
function openNew() { Object.assign(form, { province: dict.value.provinces[0] || '', name: '', source: '', type: '', specificType: '', projectYear: String(new Date().getFullYear()), responsibleDepartment: '', expansionOwner: '', leadParticipatingUnits: '', totalAmount: 0, contractAmount: 0, successDate: '', zhixinRole: '', internalSupportDepartment: '', provincialSupportDepartment: '', governmentUnit: '', stage: dict.value.stages[0], health: dict.value.healthStatuses[0], progress: '', problemDescription: '', nextPlan: '', updateCycle: dict.value.updateCycles[0], customCycleDays: 7, problemTags: [] }); note.value = ''; formMode.value = 'new'; showForm.value = true }
function openForm(mode: 'edit' | 'progress') { if (!selected.value) return; pushMobileOverlay(); Object.assign(form, JSON.parse(JSON.stringify(selected.value.project))); form.expansionOwner = ownerName(form.expansionOwner); if (form.stage && !dict.value.stages.includes(form.stage)) dict.value.stages = [...dict.value.stages, form.stage]; note.value = ''; formMode.value = mode; showForm.value = true; console.info('[项目台账] 打开表单', { mode, id: selected.value.project.id, version: form.version, health: form.health }) }
async function saveForm() {
  if (savingForm.value) return
  const id = selected.value?.project.id
  if (formMode.value !== 'new' && !id) {
    error.value = '未找到当前项目，无法保存，请关闭窗口后重新打开'
    return
  }
  savingForm.value = true
  try {
    const url = formMode.value === 'new' ? '/api/projects' : formMode.value === 'progress' ? `/api/projects/${id}/progress` : `/api/projects/${id}`
    const method = formMode.value === 'new' || formMode.value === 'progress' ? 'POST' : 'PUT'
    console.info('[项目台账] 点击保存', { mode: formMode.value, id, health: form.health, version: form.version })
    const payload = formMode.value === 'progress'
      ? { stage: form.stage, health: form.health, progress: form.progress, problemTags: form.problemTags, problemDescription: form.problemDescription, nextPlan: form.nextPlan, updateCycle: form.updateCycle, customCycleDays: form.customCycleDays, note: note.value, version: form.version }
      : { ...form, note: note.value }
    const saved = await api<Project>(url, { method, body: JSON.stringify(payload) })
    showForm.value = false
    if (!id && saved.id) {
      await openProject(saved.id)
      await loadProjects()
      return
    }
    if (!id) return
    const index = projects.value.findIndex(project => project.id === id)
    if (index >= 0) projects.value[index] = saved
    try {
      selected.value = await api<Detail>(`/api/projects/${id}`)
    } catch (refreshError) {
      error.value = `进展已保存，但详情刷新失败：${(refreshError as Error).message}`
    }
  } catch (e) {
    console.error('[项目台账] 保存失败', e)
    error.value = (e as Error).message
  } finally {
    savingForm.value = false
  }
}
function submitLogin() { login(loginForm.username.trim(), loginForm.password) }
async function openProject(id: string) { pushMobileOverlay(); selected.value = await api<Detail>(`/api/projects/${id}`) }
function reminderEnabled(stage?: string) { return stage === '策划中' || stage === '申报中' }
function status(p: Project) { if (!reminderEnabled(p.stage)) return '不提醒'; const next = Date.parse(p.nextUpdateAt || ''); if (!next || next < Date.now()) return '逾期未更新'; if (next - Date.now() < dict.value.nearDays * 86400000) return '临近更新'; return '正常' }
function formatTime(value?: string) { return value ? value.replace('T', ' ').slice(0, 16) : '暂无' }
function badgeClass(value: string) { return { good: value === '良好', blue: value === '正常', danger: value === '有问题' || value === '逾期未更新', warning: value === '临近更新' } }
async function changeView(next: 'ledger' | 'panorama' | 'mine' | 'config') { if (next !== 'panorama') disposeMap(); view.value = next; selected.value = null; showForm.value = false; correctionTarget.value = null; showExportColumns.value = false; showLedgerStats.value = false; if (next === 'panorama') { selectedMapProvince.value = ''; await loadStats() } if (next === 'config' && user.value?.role === 'admin') users.value = await api<User[]>('/api/users') }
async function saveConfig() { try { const split = (value: string) => value.split(/[，,]/).map(x => x.trim()).filter(Boolean); dict.value = await api<Dict>('/api/config', { method: 'PUT', body: JSON.stringify({ ...configDraft, provinces: split(configDraft.provinces), projectTypes: split(configDraft.projectTypes), stages: split(configDraft.stages), healthStatuses: split(configDraft.healthStatuses), problemTags: split(configDraft.problemTags), updateCycles: split(configDraft.updateCycles) }) }); error.value = '系统配置已保存' } catch (e) { error.value = (e as Error).message } }
async function clearProjects() { const beforeClearCount = total.value; try { await api<{ count?: number; cleared?: boolean }>('/api/projects', { method: 'DELETE' }); selected.value = null; selectedProjectIds.value = []; await loadProjects(); await loadStats(); if (total.value > 0) throw new Error(`清空未生效，仍有 ${total.value} 个项目，请重试`); showClearProjects.value = false; error.value = beforeClearCount ? `已清空 ${beforeClearCount} 个项目` : '项目数据已清空' } catch (e) { error.value = (e as Error).message } }
function openDeleteProject() { if (user.value?.role !== 'admin' || !selected.value) return; deleteTarget.value = selected.value.project }
async function deleteProject() {
  const target = deleteTarget.value
  if (!target?.id || deletingProject.value) return
  deletingProject.value = true
  try {
    await api(`/api/projects/${target.id}`, { method: 'DELETE' })
    projects.value = projects.value.filter(project => project.id !== target.id)
    selectedProjectIds.value = selectedProjectIds.value.filter(id => id !== target.id)
    total.value = Math.max(0, total.value - 1)
    selected.value = null
    deleteTarget.value = null
    await loadStats()
    error.value = `已删除项目“${target.name}”`
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    deletingProject.value = false
  }
}
async function toggleUser(target: User) { try { await api(`/api/users/${target.id}`, { method: 'PUT', body: JSON.stringify({ status: target.status === '启用' ? '停用' : '启用' }) }); target.status = target.status === '启用' ? '停用' : '启用' } catch (e) { error.value = (e as Error).message } }
async function createUser() { try { const created = await api<User>('/api/users', { method: 'POST', body: JSON.stringify(newUser) }); users.value.push(created); showUserForm.value = false; Object.assign(newUser, { name: '', username: '', password: '', role: 'user' }) } catch (e) { error.value = (e as Error).message } }
function openPasswordForm() { showAccountMenu.value = false; Object.assign(passwordForm, { currentPassword: '', newPassword: '', confirmPassword: '' }); showPasswordForm.value = true }
async function changePassword() { if (passwordForm.newPassword !== passwordForm.confirmPassword) { error.value = '两次输入的新密码不一致'; return }; try { await api('/api/me/password', { method: 'PUT', body: JSON.stringify({ currentPassword: passwordForm.currentPassword, newPassword: passwordForm.newPassword }) }); showPasswordForm.value = false; error.value = '密码已修改，请牢记新密码' } catch (e) { error.value = (e as Error).message } }
function openResetPassword(target: User) { resetTarget.value = target; resetPassword.value = '' }
async function resetUserPassword() { if (!resetTarget.value) return; try { await api(`/api/users/${resetTarget.value.id}`, { method: 'PUT', body: JSON.stringify({ password: resetPassword.value }) }); resetTarget.value = null; error.value = '密码已重置' } catch (e) { error.value = (e as Error).message } }
function openCorrection(history: History) { correctionTarget.value = history; Object.assign(correction, { progress: history.progress || '', problemDescription: history.problemDescription || '', nextPlan: history.nextPlan || '', note: history.note || '', correctionNote: '' }) }
async function saveCorrection() { if (!selected.value || !correctionTarget.value) return; try { await api(`/api/projects/${selected.value.project.id}/history/${correctionTarget.value.id}/correction`, { method: 'POST', body: JSON.stringify(correction) }); selected.value = await api<Detail>(`/api/projects/${selected.value.project.id}`); correctionTarget.value = null } catch (e) { error.value = (e as Error).message } }
function csvValue(value: unknown) { return value == null ? '' : String(value) }
const ownerNameMap: Record<string, string> = { liujingwen: '刘景文', tianyu: '田羽', yuquanqing: '蔚泉清', wangchen: '王晨', qinxiaomin: '秦晓敏', luyuhua: '卢玉华' }
function ownerName(value: unknown) { const text = csvValue(value).trim(); return ownerNameMap[text.toLowerCase()] || text }
function importHeader(value: unknown) { return csvValue(value).replace(/[\s\u00a0\u3000\ufeff]/g, '').replace(/[（）]/g, '') }
function importRowValue(row: Record<string, unknown>, ...headers: string[]) {
  for (const header of headers) {
    const key = Object.keys(row).find(item => importHeader(item) === importHeader(header))
    if (key && row[key] !== '' && row[key] != null) return row[key]
  }
  return ''
}
function exportColumnWidths(rows: Record<string, unknown>[]) { return Object.keys(rows[0] || {}).map(key => ({ wch: Math.min(32, Math.max(10, Math.max(key.length, ...rows.slice(0, 80).map(row => String(row[key] || '').length)) + 2)) })) }
function exportProjectType(project: Project) { return project.type?.trim() || '未分类' }
function compareExportProjects(a: Project, b: Project) {
  const aType = exportProjectType(a)
  const bType = exportProjectType(b)
  const aTypeIndex = exportProjectTypeOrder.indexOf(aType)
  const bTypeIndex = exportProjectTypeOrder.indexOf(bType)
  const typeOrder = (aTypeIndex < 0 ? exportProjectTypeOrder.length : aTypeIndex) - (bTypeIndex < 0 ? exportProjectTypeOrder.length : bTypeIndex)
  if (typeOrder) return typeOrder
  if (aTypeIndex < 0 && aType !== bType) return aType.localeCompare(bType, 'zh-CN')
  const yearOrder = String(b.projectYear || '').localeCompare(String(a.projectYear || ''), 'zh-CN', { numeric: true })
  return yearOrder || String(a.name || '').localeCompare(String(b.name || ''), 'zh-CN')
}
function exportDate(value: unknown) {
  if (value == null || value === '') return ''
  if (typeof value === 'number' && Number.isFinite(value)) {
    const parsed = XLSX.SSF.parse_date_code(value)
    if (parsed) return `${parsed.y}-${String(parsed.m).padStart(2, '0')}-${String(parsed.d).padStart(2, '0')}`
  }
  const text = String(value)
  const match = text.match(/^(\d{4})[-\/]?(\d{1,2})[-\/]?(\d{1,2})/)
  return match ? `${match[1]}-${match[2].padStart(2, '0')}-${match[3].padStart(2, '0')}` : text
}
function importDate(value: unknown) { return exportDate(value) }
function openExportColumns() { exportColumnKeys.value = [...defaultExportColumnKeys]; showExportColumns.value = true }
function toggleExportColumn(key: string) { exportColumnKeys.value = exportColumnKeys.value.includes(key) ? exportColumnKeys.value.filter(item => item !== key) : [...exportColumnKeys.value, key] }
async function chooseImport(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const workbook = XLSX.read(await file.arrayBuffer(), { type: 'array' })
  const rows = XLSX.utils.sheet_to_json<Record<string, unknown>>(workbook.Sheets[workbook.SheetNames[0]], { defval: '' })
  importRows.value = rows.map(row => ({
    province: csvValue(importRowValue(row, '省份')), name: csvValue(importRowValue(row, '项目名称')), source: csvValue(importRowValue(row, '来源')),
    projectYear: csvValue(importRowValue(row, '项目年份')) || String(new Date().getFullYear()), type: csvValue(importRowValue(row, '项目类型')),
    specificType: csvValue(importRowValue(row, '具体类型', '项目具体类型')), responsibleDepartment: csvValue(importRowValue(row, '主责部门')),
    expansionOwner: ownerName(importRowValue(row, '拓展组负责人', '拓展组项目负责人')), leadParticipatingUnits: csvValue(importRowValue(row, '项目牵头参与单位', '牵头参与单位')),
    totalAmount: Number(importRowValue(row, '项目总金额', '项目总金额（万元）')) || 0, contractAmount: Number(importRowValue(row, '智芯合同金额', '智芯合同金额（万元）')) || 0,
    successDate: importDate(importRowValue(row, '申报成功时间')), zhixinRole: csvValue(importRowValue(row, '智芯角色')),
    internalSupportDepartment: csvValue(importRowValue(row, '智芯内部支撑部门', '内部支撑部门')), provincialSupportDepartment: csvValue(importRowValue(row, '省公司支撑部门')),
    governmentUnit: csvValue(importRowValue(row, '政府主管单位或其他相关单位', '政府主管单位')), stage: csvValue(importRowValue(row, '项目阶段', '项目状态')) || '策划中',
    health: csvValue(importRowValue(row, '健康状态')) || '暂无信息', problemTags: csvValue(importRowValue(row, '问题标签')).split(/[、,，]/).filter(Boolean),
    progress: csvValue(importRowValue(row, '当前进展', '项目进展')), problemDescription: csvValue(importRowValue(row, '问题说明')),
    nextPlan: csvValue(importRowValue(row, '下一步计划')), updateCycle: csvValue(importRowValue(row, '更新周期')) || '每周',
    customCycleDays: Number(importRowValue(row, '自定义周期天数')) || 0,
  }))
  importResult.value = await api('/api/imports/projects', { method: 'POST', body: JSON.stringify({ projects: importRows.value, confirm: false }) })
  input.value = ''
}
async function confirmImport() { importResult.value = { ...await api('/api/imports/projects', { method: 'POST', body: JSON.stringify({ projects: importRows.value, confirm: true }) }), done: true }; filters.page = 1; await loadProjects() }
async function exportLedger() {
  if (!exportColumnKeys.value.length) { error.value = '请至少选择一列'; return }
  const q = new URLSearchParams()
  Object.entries(filters).forEach(([k, v]) => { if (k !== 'page' && k !== 'pageSize') { const value = Array.isArray(v) ? v.join(',') : String(v); if (value) q.set(k, value) } })
  const rows = selectedProjectIds.value.length
    ? projects.value.filter(project => selectedProjectIds.value.includes(project.id || ''))
    : await api<Project[]>(`/api/exports/projects?${q}`)
  const selectedColumns = exportColumns.filter(([key]) => exportColumnKeys.value.includes(key))
  const output: Record<string, unknown>[] = []
  let previousType = ''
  for (const p of [...rows].sort(compareExportProjects)) {
    const type = exportProjectType(p)
    if (previousType && type !== previousType) output.push({})
    const values: Record<string, unknown> = { province: p.province, name: p.name, type: p.type, specificType: p.specificType, projectYear: p.projectYear, responsibleDepartment: p.responsibleDepartment, expansionOwner: p.expansionOwner, leadParticipatingUnits: p.leadParticipatingUnits, totalAmount: p.totalAmount, contractAmount: p.contractAmount, source: p.source, successDate: exportDate(p.successDate), zhixinRole: p.zhixinRole, internalSupportDepartment: p.internalSupportDepartment, provincialSupportDepartment: p.provincialSupportDepartment, governmentUnit: p.governmentUnit, stage: p.stage, health: p.health, problemTags: (p.problemTags || []).join('、'), progress: p.progress, problemDescription: p.problemDescription, nextPlan: p.nextPlan, updateCycle: p.updateCycle }
    output.push(Object.fromEntries(selectedColumns.map(([key, title]) => [title, values[key]])))
    previousType = type
  }
  const headers = selectedColumns.map(([, title]) => title)
  const sheetRows = output.map(row => headers.map(header => row[header] ?? ''))
  const book = XLSX.utils.book_new()
  const sheet = XLSX.utils.aoa_to_sheet([headers, ...sheetRows])
  sheet['!cols'] = exportColumnWidths(output.filter(row => Object.keys(row).length))
  XLSX.utils.book_append_sheet(book, sheet, '项目台账')
  XLSX.writeFile(book, `项目台账-${new Date().toISOString().slice(0, 10)}.xlsx`)
  showExportColumns.value = false
}
async function exportHistory() { if (!selected.value) return; const rows = await api<History[]>(`/api/exports/history?projectId=${selected.value.project.id}`); const output = rows.map(h => ({ 更新时间: formatTime(h.updatedAt), 更新人: h.updatedBy, 项目阶段: h.stage, 健康状态: h.health, 问题标签: (h.problemTags || []).join('、'), 进展: h.progress, 问题说明: h.problemDescription, 下一步计划: h.nextPlan, 备注: h.note, 修正说明: h.correctionNote })); const book = XLSX.utils.book_new(); XLSX.utils.book_append_sheet(book, XLSX.utils.json_to_sheet(output), '进展历史'); XLSX.writeFile(book, `项目历史-${selected.value.project.name}.xlsx`) }
const mapElement = ref<HTMLElement | null>(null)
let mapChart: echarts.ECharts | null = null
const mapProvinceNames = computed(() => new Set((chinaGeoJson as GeoJSON.FeatureCollection).features.map(feature => feature.properties?.name).filter(Boolean)))

function renderMap() {
  if (!mapElement.value) return
  if (mapChart?.getDom() !== mapElement.value) {
    mapChart?.dispose()
    mapChart = echarts.init(mapElement.value)
  }
  echarts.registerMap('china', chinaGeoJson as Parameters<typeof echarts.registerMap>[1])
  const data = visibleStats.value.filter(item => mapProvinceNames.value.has(item.province)).map(item => ({
    name: item.province,
    value: item.total,
    issues: item.issues,
    itemStyle: item.issues > 0
      ? { areaColor: '#c94d49', borderColor: '#ffc0bb', borderWidth: 1.6 }
      : item.total > 0
        ? { areaColor: '#328e99', borderColor: '#8de3d1', borderWidth: 1 }
        : { areaColor: '#172f3a', borderColor: '#50717a', borderWidth: 0.8 },
    emphasis: { itemStyle: { areaColor: item.issues > 0 ? '#e25d57' : '#f0bd61' } }
  }))
  mapChart.setOption({
    tooltip: { trigger: 'item', formatter: (params: { name: string; value?: number; data?: { issues?: number } }) => `${params.name}<br/>项目 ${params.value || 0} 个 · 问题 ${params.data?.issues || 0} 个` },
    series: [{ name: '项目数量', type: 'map', map: 'china', roam: true, selectedMode: false, data, itemStyle: { areaColor: '#172f3a', borderColor: '#50717a', borderWidth: 0.8 }, emphasis: { label: { show: true, color: '#fff' } }, label: { show: false } }]
  })
  mapChart.off('click')
  mapChart.on('click', (params: { name?: string }) => { if (params.name && mapProvinceNames.value.has(params.name)) selectMapProvince(params.name) })
  requestAnimationFrame(() => mapChart?.resize())
}
function resizeMap() { mapChart?.resize() }
function disposeMap() { mapChart?.dispose(); mapChart = null }
function closeAccountMenu(event: MouseEvent) { if (accountMenu.value && !accountMenu.value.contains(event.target as Node)) showAccountMenu.value = false; if (stageMenu.value && !stageMenu.value.contains(event.target as Node)) showStageMenu.value = false }
function handleMobileBack() { if (!window.matchMedia('(max-width: 800px)').matches) return; if (showForm.value) { showForm.value = false; return }; if (selected.value) { selected.value = null; return } }
onMounted(() => { boot(); window.addEventListener('resize', resizeMap); window.addEventListener('popstate', handleMobileBack); document.addEventListener('click', closeAccountMenu) })
watch([view, stats], async () => { if (view.value === 'panorama') { await nextTick(); renderMap() } }, { deep: true })
onBeforeUnmount(() => { window.removeEventListener('resize', resizeMap); window.removeEventListener('popstate', handleMobileBack); document.removeEventListener('click', closeAccountMenu); disposeMap() })
const visibleStats = computed(() => dict.value.provinces.map(province => stats.value.find(x => x.province === province) || { province, total: 0, issues: 0 }))
const totalProjects = computed(() => visibleStats.value.reduce((sum, item) => sum + item.total, 0))
const totalIssues = computed(() => visibleStats.value.reduce((sum, item) => sum + item.issues, 0))
const coveredProvinces = computed(() => visibleStats.value.filter(item => item.total > 0).length)
const rankedProvinces = computed(() => [...visibleStats.value].filter(item => item.total > 0).sort((a, b) => b.total - a.total).slice(0, 5))
const issueProvinces = computed(() => [...visibleStats.value].filter(item => item.issues > 0).sort((a, b) => b.issues - a.issues).slice(0, 5))
const selectedMapProvince = ref('')
const selectedMapLocation = computed(() => visibleStats.value.find(item => item.province === selectedMapProvince.value) || null)
const selectedProjectTotal = computed(() => selectedMapLocation.value ? selectedMapLocation.value.total : totalProjects.value)
const selectedProjectIssues = computed(() => selectedMapLocation.value ? selectedMapLocation.value.issues : totalIssues.value)
const selectedIssueRate = computed(() => selectedProjectTotal.value ? Math.round(selectedProjectIssues.value / selectedProjectTotal.value * 100) : 0)
const selectedProjectRank = computed(() => selectedMapLocation.value?.total ? [...visibleStats.value].filter(item => item.total > 0 && item.total > selectedProjectTotal.value).length + 1 : 0)
function selectMapProvince(province: string) { selectedMapProvince.value = province }
function openMapLedger() {
  const province = selectedMapLocation.value?.province || ''
  Object.assign(filters, { q: '', province, stage: [], health: '', type: '', specificType: '', expansionOwner: '', projectYear: '', successDateFrom: '', successDateTo: '', quick: '', sort: '', page: 1 })
  selectedProjectIds.value = []
  showLedgerStats.value = false
  changeView('ledger')
  loadProjects()
}
</script>

<template>
  <div v-if="!user" class="login-page"><div class="login-art" aria-hidden="true"></div><form class="login-panel" @submit.prevent="submitLogin"><div class="section-kicker">项目台账</div><h2>登录</h2><label>账号<input v-model="loginForm.username" @input="error = ''" required autocomplete="username" /></label><label>密码<input v-model="loginForm.password" @input="error = ''" type="password" required autocomplete="current-password" /></label><p v-if="error" class="form-error">{{ error }}</p><button class="primary wide" :disabled="loading">{{ loading ? '登录中...' : '登录' }}</button></form></div>
  <div v-else class="app-shell">
    <header class="topbar"><div class="brand"><span class="brand-mark">项</span><div><strong>项目台账</strong><small>项目管理系统</small></div></div><nav><button :class="{ active: view === 'ledger' }" @click="changeView('ledger')">项目台账</button><button :class="{ active: view === 'panorama' }" @click="changeView('panorama')">全国全景</button><button v-if="user.role === 'admin'" :class="{ active: view === 'config' }" @click="changeView('config')">系统配置</button></nav><div ref="accountMenu" class="account account-menu-wrap"><button class="avatar-button" :aria-expanded="showAccountMenu" aria-label="打开账户菜单" @click="showAccountMenu = !showAccountMenu">{{ user.name.slice(0, 1) }}</button><div v-if="showAccountMenu" class="account-menu"><strong>{{ user.name }}</strong><small>{{ user.role === 'admin' ? '管理员' : '普通用户' }}</small><button @click="showAccountMenu = false; changeView('mine')">我的</button><button @click="showAccountMenu = false; openPasswordForm">修改密码</button><button @click="logout">退出</button></div></div></header>
      <main class="main-content" :class="{ 'ledger-content': view === 'ledger' }">
      <div v-if="error" class="toast">{{ error }} <button @click="error = ''">关闭</button></div>
      <template v-if="view === 'ledger'"><section class="page-heading"><div><h1>项目台账</h1><p>{{ total }} 个项目<span v-if="total"> · 点击项目查看详情</span></p></div><div class="heading-actions"><label class="file-button">导入<input type="file" accept=".xlsx,.xls,.csv" @change="chooseImport" /></label><button @click="openExportColumns">导出</button><button @click="showLedgerStats = !showLedgerStats">数据统计</button><button v-if="user.role === 'admin' || user.role === 'user'" class="primary" @click="openNew">＋ 新增项目</button></div></section><section class="filter-bar"><input class="filter-search" v-model="filters.q" placeholder="搜索项目名称或省份" @keyup.enter="applyFilters" /><select class="province-filter" v-model="filters.province"><option value="">全部</option><option value="__all_provinces__">全部省份</option><option value="__all_direct_units__">全部直属单位</option><optgroup label="省份"><option v-for="x in provinceOptions" :key="x">{{ x }}</option></optgroup><optgroup label="直属单位"><option v-for="x in directUnitOptions" :key="x">{{ x }}</option></optgroup></select><select class="year-filter" v-model="filters.projectYear"><option value="">全部年份</option><option v-for="year in [...new Set(projects.map(p => p.projectYear))].sort().reverse()" :key="year">{{ year }}</option></select><select class="type-filter" v-model="filters.type"><option value="">全部类型</option><option v-for="x in dict.projectTypes" :key="x">{{ x }}</option></select><select class="owner-filter" v-model="filters.expansionOwner"><option value="">全部负责人</option><option v-for="owner in ownerOptions" :key="owner">{{ owner }}</option></select><div ref="stageMenu" class="stage-filter stage-picker"><button type="button" class="stage-picker-trigger" @click="showStageMenu = !showStageMenu"><span v-if="!filters.stage.length" class="stage-placeholder">全部阶段</span><span v-for="(stage, index) in filters.stage" v-show="index < 3" :key="stage" class="stage-tag">{{ stage }}<i @click.stop="removeStage(stage)">×</i></span><span v-if="filters.stage.length > 3" class="stage-more">+{{ filters.stage.length - 3 }}</span><b>⌄</b></button><div v-if="showStageMenu" class="stage-picker-menu"><div v-for="x in dict.stages" :key="x" class="stage-option"><input type="checkbox" :checked="filters.stage.includes(x)" @change="toggleStage(x)" /><span>{{ x }}</span></div></div></div><select class="health-filter" v-model="filters.health"><option value="">全部健康状态</option><option v-for="x in dict.healthStatuses" :key="x">{{ x }}</option></select><div class="success-date-filter"><span>申报成功时间</span><input v-model="filters.successDateFrom" type="date" aria-label="申报成功时间起" /><i>至</i><input v-model="filters.successDateTo" type="date" aria-label="申报成功时间止" /></div><button class="filter-submit" @click="applyFilters">筛选</button><button class="reset-button" @click="resetFilters">重置</button></section><section v-if="showLedgerStats" class="ledger-stats"><div class="stats-summary"><strong>{{ statisticProjects.length }}</strong><span>{{ selectedProjectIds.length ? '已选项目' : '当前筛选项目' }}</span><strong>{{ ledgerTotalAmount.toFixed(2) }}</strong><span>项目总金额（万元）</span><strong>{{ ledgerContractAmount.toFixed(2) }}</strong><span>智芯合同金额（万元）</span></div><table><thead><tr><th>项目类型</th><th>项目数</th><th>项目总金额（万元）</th><th>智芯合同金额（万元）</th></tr></thead><tbody><tr v-for="item in ledgerStats" :key="item.type"><td>{{ item.type }}</td><td>{{ item.count }}</td><td>{{ item.totalAmount.toFixed(2) }}</td><td>{{ item.contractAmount.toFixed(2) }}</td></tr><tr v-if="!ledgerStats.length"><td colspan="4">当前筛选结果暂无数据</td></tr></tbody></table></section><div v-if="loading" class="loading">正在加载项目...</div><div class="table-card"><table><colgroup><col class="col-province" /><col class="col-name" /><col class="col-year" /><col class="col-type" /><col class="col-stage" /><col class="col-health" /><col class="col-progress" /><col class="col-owner" /><col class="col-status" /><col class="col-updated" /></colgroup><thead><tr><th>省份</th><th>项目名称</th><th>年份</th><th>项目类型</th><th>阶段</th><th>健康</th><th>最新进展</th><th>负责人</th><th>更新状态</th><th>最近更新时间</th></tr></thead><tbody class="desktop-projects"><tr v-for="p in projects" :key="p.id" @click="openProject(p.id!)"><td class="sticky-province">{{ p.province }}</td><td class="sticky-name"><strong>{{ p.name }}</strong></td><td>{{ p.projectYear }}</td><td>{{ p.type || '未填写' }}</td><td><span class="badge neutral">{{ p.stage }}</span></td><td><span class="badge" :class="badgeClass(p.health)">{{ p.health }}</span></td><td class="progress-cell">{{ p.progress || '暂无进展' }}</td><td>{{ p.expansionOwner || '未填写' }}</td><td><span class="badge" :class="badgeClass(status(p))">{{ status(p) }}</span></td><td>{{ formatTime(p.updatedAt) }}<small>{{ p.updatedBy || '暂无' }}</small></td></tr></tbody><tbody class="mobile-projects"><tr v-for="p in projects" :key="p.id" @click="openProject(p.id!)"><td><strong>{{ p.name }}</strong><small>{{ p.province }} · {{ p.projectYear }} · {{ p.type || '未填写' }}</small><p>{{ p.progress || '暂无进展' }}</p><span class="badge neutral">{{ p.stage }}</span> <span class="badge" :class="badgeClass(p.health)">{{ p.health }}</span> <span class="badge" :class="badgeClass(status(p))">{{ status(p) }}</span><small>负责人 {{ p.expansionOwner || '未填写' }} · 更新于 {{ formatTime(p.updatedAt) }}</small></td></tr></tbody><tbody v-if="!projects.length"><tr><td colspan="10" class="empty">暂无项目数据</td></tr></tbody></table></div></template>
      <section v-if="view === 'ledger'" class="ledger-v2"><div class="table-card ledger-table-scroll"><table><colgroup><col v-for="key in Object.keys(columnWidths)" :key="key" :style="{ width: `${columnWidths[key]}px` }" /></colgroup><thead><tr><th v-for="item in [{ key: 'selected', label: '选择' }, { key: 'province', label: '省份' }, { key: 'name', label: '项目名称' }, { key: 'projectYear', label: '年份' }, { key: 'type', label: '项目类型' }, { key: 'specificType', label: '具体类型' }, { key: 'stage', label: '阶段' }, { key: 'health', label: '健康' }, { key: 'progress', label: '最新进展' }, { key: 'expansionOwner', label: '负责人' }, { key: 'status', label: '更新状态' }, { key: 'updatedAt', label: '最近更新时间' }]" :key="item.key" :class="[`sort-${item.key}`, { 'is-sorted': sortKey === item.key }]" @click="item.key !== 'selected' && setSort(item.key as typeof sortKey)"><input v-if="item.key === 'selected'" type="checkbox" :checked="sortedProjects.length > 0 && sortedProjects.every(project => selectedProjectIds.includes(project.id || ''))" @click.stop="toggleAllProjects" /><span v-else>{{ item.label }}<b v-if="sortKey === item.key">{{ sortDirection === 'asc' ? ' ↑' : ' ↓' }}</b></span><i class="resize-handle" @pointerdown.stop.prevent="startResize($event, item.key)"></i></th></tr></thead><tbody><tr v-for="p in sortedProjects" :key="p.id" @click="openProject(p.id!)"><td class="select-cell"><input type="checkbox" :checked="selectedProjectIds.includes(p.id || '')" @click.stop="toggleProject(p.id)" /></td><td class="sticky-province">{{ p.province }}</td><td class="sticky-name"><strong>{{ p.name }}</strong></td><td>{{ p.projectYear }}</td><td>{{ p.type || '未填写' }}</td><td>{{ p.specificType || '未填写' }}</td><td><span class="badge neutral">{{ p.stage }}</span></td><td><span class="badge" :class="badgeClass(p.health)">{{ p.health }}</span></td><td class="progress-cell">{{ p.progress || '暂无进展' }}</td><td>{{ p.expansionOwner || '未填写' }}</td><td><span class="badge" :class="badgeClass(status(p))">{{ status(p) }}</span></td><td>{{ formatTime(p.updatedAt) }}<small>{{ p.updatedBy || '暂无' }}</small></td></tr><tr v-if="!sortedProjects.length"><td colspan="12" class="empty">暂无项目数据</td></tr></tbody></table></div></section>
      <section v-if="view === 'ledger'" class="mobile-ledger-list">
        <article v-for="p in sortedProjects" :key="p.id" class="project-card" @click="openProject(p.id!)">
          <div class="project-card-head"><strong>{{ p.name }}</strong><button class="quick-update" @click.stop="openProject(p.id!).then(() => openForm('progress'))">更新</button></div>
          <small>{{ p.province }} · {{ p.projectYear }} · {{ p.type || '未填写' }}</small>
          <p>{{ p.progress || '暂无进展' }}</p>
          <div class="project-card-statuses">
            <span class="project-card-status"><small>阶段</small><span class="badge neutral">{{ p.stage }}</span></span>
            <span class="project-card-status"><small>健康</small><span class="badge" :class="badgeClass(p.health)">{{ p.health }}</span></span>
            <span class="project-card-status"><small>更新</small><span class="badge" :class="badgeClass(status(p))">{{ status(p) }}</span></span>
          </div>
          <small>负责人 {{ p.expansionOwner || '未填写' }} · 更新于 {{ formatTime(p.updatedAt) }}</small>
        </article>
        <p v-if="!sortedProjects.length" class="empty">暂无项目数据</p>
      </section>
    </main>
    <section v-if="view === 'panorama'" class="map-overlay">
      <div class="panorama-topbar"><div><div class="section-kicker">全国运营态势 / 实时</div><h1>全国项目态势</h1></div><div class="panorama-meta"><span>数据实时汇总</span><button @click="changeView('ledger')">返回台账</button></div></div>
      <div class="panorama-kpis">
        <div class="panorama-kpi"><small>项目总数</small><strong>{{ totalProjects }}</strong><span>全国台账项目</span></div>
        <div class="panorama-kpi"><small>项目省份</small><strong>{{ coveredProvinces }}<em>/ {{ visibleStats.length }}</em></strong><span>已形成项目覆盖</span></div>
        <div class="panorama-kpi risk"><small>问题项目</small><strong>{{ totalIssues }}</strong><span>需要重点关注</span></div>
        <div class="panorama-kpi"><small>健康项目</small><strong>{{ totalProjects - totalIssues }}</strong><span>暂无问题记录</span></div>
      </div>
      <div class="map-scene"><div ref="mapElement" class="map-surface real-map" role="img" aria-label="中国省级项目地图"></div><aside class="map-summary"><div class="province-summary-head"><div><div class="section-kicker">省份详情</div><h2>{{ selectedMapLocation?.province || '全国项目概览' }}</h2></div><span class="summary-state">{{ selectedMapLocation ? '已选省份' : '全国汇总' }}</span></div><div class="province-total"><strong>{{ selectedProjectTotal }}</strong><span>项目总数</span><em v-if="selectedMapLocation && selectedMapLocation.total">全国第 {{ selectedProjectRank }} 名</em><em v-else-if="selectedMapLocation">暂无排名</em><em v-else>全国项目汇总</em></div><div class="province-metrics"><div class="metric"><small>项目规模</small><b>{{ selectedMapLocation ? (selectedMapLocation.total ? `第 ${selectedProjectRank} 名` : '暂无排名') : `${coveredProvinces} 个省份` }}</b><span>{{ selectedMapLocation ? '按项目数量排序' : '已形成项目覆盖' }}</span></div><div class="metric risk-metric"><small>风险概况</small><b>{{ selectedMapLocation ? `${selectedProjectIssues} 个` : `${totalIssues} 个` }}</b><span>{{ selectedMapLocation ? `问题率 ${selectedIssueRate}%` : '全国问题项目' }}</span></div></div><button class="map-link" @click="openMapLedger">{{ selectedMapLocation ? '查看该省台账' : '查看项目台账' }}</button><p v-if="!selectedMapLocation" class="summary-hint">点击地图中的省份，查看该省项目结构与风险状态</p></aside></div>
      <div class="panorama-bottom"><section class="ranking-panel"><div class="panel-heading"><span>项目规模</span><b>项目数量排行</b></div><button v-for="(item, index) in rankedProvinces" :key="item.province" @click="selectMapProvince(item.province)" class="ranking-row"><i>{{ String(index + 1).padStart(2, '0') }}</i><span>{{ item.province }}</span><div class="ranking-bar"><b :style="{ width: `${(item.total / (rankedProvinces[0]?.total || 1)) * 100}%` }"></b></div><strong>{{ item.total }}</strong></button><p v-if="!rankedProvinces.length" class="ranking-empty">暂无项目数据</p></section><section class="ranking-panel issue-ranking"><div class="panel-heading"><span>风险关注</span><b>问题项目省份</b></div><button v-for="item in issueProvinces" :key="item.province" @click="selectMapProvince(item.province)" class="ranking-row"><span>{{ item.province }}</span><div class="ranking-bar"><b :style="{ width: `${(item.issues / (issueProvinces[0]?.issues || 1)) * 100}%` }"></b></div><strong>{{ item.issues }} 个</strong></button><p v-if="!issueProvinces.length" class="ranking-empty">当前暂无问题项目</p></section></div>
    </section>
    <div v-if="selected" class="history-tools"><button @click="exportHistory">导出历史</button><template v-if="user.role === 'admin' || user.role === 'user'"><span>历史修正</span><button v-for="h in selected.history" :key="h.id" @click="openCorrection(h)">{{ formatTime(h.updatedAt) }} · 修正</button></template></div>
    <aside v-if="selected" class="drawer-backdrop" @click.self="selected = null"><section class="drawer"><button class="close-button" @click="selected = null">×</button><div class="section-kicker">{{ selected.project.province }} · {{ selected.project.projectYear }}</div><h2>{{ selected.project.name }}</h2><div class="drawer-actions"><button v-if="user.role === 'admin' || user.role === 'user'" class="primary" @click="openForm('progress')">更新进展</button><button v-if="user.role === 'admin' || user.role === 'user'" @click="openForm('edit')">编辑基本信息</button><button v-if="user.role === 'admin'" class="danger-button" @click="openDeleteProject">删除项目</button></div><div class="drawer-scroll"><section class="detail-section"><h3>基本信息</h3><dl><dt>省份 / 年份</dt><dd>{{ selected.project.province }} · {{ selected.project.projectYear }}</dd><dt>来源</dt><dd>{{ selected.project.source || '未填写' }}</dd><dt>项目类型</dt><dd>{{ selected.project.type || '未填写' }} {{ selected.project.specificType ? `· ${selected.project.specificType}` : '' }}</dd><dt>主责部门</dt><dd>{{ selected.project.responsibleDepartment || '未填写' }}</dd><dt>拓展组负责人</dt><dd>{{ selected.project.expansionOwner || '未填写' }}</dd><dt>牵头参与单位</dt><dd>{{ selected.project.leadParticipatingUnits || '未填写' }}</dd><dt>申报成功时间</dt><dd>{{ selected.project.successDate || '未填写' }}</dd><dt>政府主管单位</dt><dd>{{ selected.project.governmentUnit || '未填写' }}</dd></dl></section><section class="detail-section"><h3>金额与支撑</h3><dl><dt>项目总金额</dt><dd>{{ selected.project.totalAmount || 0 }} 万元</dd><dt>智芯合同金额</dt><dd>{{ selected.project.contractAmount || 0 }} 万元</dd><dt>智芯角色</dt><dd>{{ selected.project.zhixinRole || '未填写' }}</dd><dt>智芯内部支撑部门</dt><dd>{{ selected.project.internalSupportDepartment || '未填写' }}</dd><dt>省公司支撑部门</dt><dd>{{ selected.project.provincialSupportDepartment || '未填写' }}</dd></dl></section><section class="detail-section"><h3>当前状态</h3><dl><dt>阶段 / 健康</dt><dd><span class="badge neutral">{{ selected.project.stage }}</span> <span class="badge" :class="badgeClass(selected.project.health)">{{ selected.project.health }}</span></dd><dt>问题标签</dt><dd>{{ (selected.project.problemTags || []).join('、') || '无' }}</dd><dt>当前进展</dt><dd>{{ selected.project.progress || '暂无进展' }}</dd><dt>问题说明</dt><dd>{{ selected.project.problemDescription || '无' }}</dd><dt>下一步计划</dt><dd>{{ selected.project.nextPlan || '未填写' }}</dd><dt>更新周期</dt><dd>{{ selected.project.updateCycle }} · 下次 {{ formatTime(selected.project.nextUpdateAt) }} · {{ status(selected.project) }}</dd><dt>最近更新</dt><dd>{{ formatTime(selected.project.updatedAt) }} · {{ selected.project.updatedBy || '未记录' }}</dd></dl></section><section class="detail-section"><h3>进展历史</h3><div class="timeline"><article v-for="h in selected.history" :key="h.id"><time>{{ formatTime(h.updatedAt) }}</time><b>{{ h.updatedBy }}</b><p>{{ h.stage }} · {{ h.health }}<br />{{ h.progress || '暂无进展' }}</p><small>{{ h.problemDescription }} {{ h.note }} {{ h.correctionNote ? `（${h.correctionNote}）` : '' }}</small></article></div></section></div></section></aside>
    <div v-if="showForm" class="modal-backdrop"><form class="modal" :class="{ 'progress-form': formMode === 'progress' }" @submit.prevent="saveForm"><button type="button" class="close-button" @click="showForm = false">×</button><div class="section-kicker">{{ formMode === 'new' ? '新建项目' : formMode === 'progress' ? '进展更新' : '编辑项目' }}</div><h2>{{ formMode === 'new' ? '新增项目' : formMode === 'progress' ? '更新进展' : '编辑项目' }}</h2><div class="form-grid"><label>项目阶段<select v-model="form.stage"><option v-for="x in dict.stages" :key="x">{{ x }}</option></select></label><label>健康状态<select v-model="form.health"><option v-for="x in dict.healthStatuses" :key="x">{{ x }}</option></select></label><label class="wide-field">当前进展<textarea v-model="form.progress" /></label><label v-if="form.health === '有问题'" class="wide-field">问题说明<textarea v-model="form.problemDescription" /></label><div v-if="form.health === '有问题'" class="wide-field tag-field"><span>问题标签</span><label v-for="x in dict.problemTags" :key="x" class="check-label"><input type="checkbox" :value="x" v-model="form.problemTags" />{{ x }}</label></div><label class="wide-field">下一步计划<textarea v-model="form.nextPlan" /></label><label>更新周期<select v-model="form.updateCycle"><option v-for="x in dict.updateCycles" :key="x">{{ x }}</option></select></label><label v-if="form.updateCycle === '自定义'">周期天数<input v-model.number="form.customCycleDays" type="number" min="1" /></label><label class="wide-field">本次备注<textarea v-model="note" /></label></div><div class="modal-actions"><button type="button" @click="showForm = false">取消</button><button type="submit" class="primary" :disabled="savingForm">{{ savingForm ? '保存中...' : '保存' }}</button></div></form></div>
    <section v-if="view === 'config'" class="config-overlay"><div class="config-sheet"><div class="section-kicker">系统管理 / 03</div><h1>系统配置</h1><div class="config-columns"><section class="config-panel user-panel"><div class="panel-title"><div><h3>用户与权限</h3><p>管理登录状态，并为用户设置新密码。</p></div><button class="primary" @click="showUserForm = true">＋ 新增用户</button></div><div class="user-list"><article v-for="item in users" :key="item.id" class="user-row"><div class="user-avatar">{{ item.name.slice(0, 1).toUpperCase() }}</div><div class="user-identity"><strong>{{ item.name }}</strong><small>{{ item.username }}</small></div><span class="role-label">{{ item.role === 'admin' ? '管理员' : '普通用户' }}</span><button class="status-button" :class="{ stopped: item.status === '停用' }" @click="toggleUser(item)">{{ item.status }}</button><button class="reset-button" @click="openResetPassword(item)">重置密码</button></article></div></section><form class="config-panel dictionary-panel" @submit.prevent="saveConfig"><div class="panel-title"><div><h3>项目字典</h3><p>用逗号或顿号分隔选项，保存后立即应用到项目表单。</p></div><span class="dictionary-count">{{ configDraft.stages.split(/[，,]/).filter((x: string) => x.trim()).length }} 组配置</span></div><div class="dictionary-grid"><label><span>省份 <em>项目所属地区</em></span><textarea v-model="configDraft.provinces" placeholder="北京，上海，江苏" /></label><label><span>项目类型 <em>项目分类</em></span><textarea v-model="configDraft.projectTypes" placeholder="政府项目，国网项目，科技项目" /></label><label><span>项目阶段 <em>流程状态</em></span><textarea v-model="configDraft.stages" placeholder="策划中，申报中，已立项" /></label><label><span>健康状态 <em>风险判断</em></span><textarea v-model="configDraft.healthStatuses" placeholder="良好，正常，有问题" /></label><label><span>问题标签 <em>问题分类</em></span><textarea v-model="configDraft.problemTags" placeholder="进度滞后，材料缺失" /></label><label><span>更新周期 <em>提醒频率</em></span><textarea v-model="configDraft.updateCycles" placeholder="每周，每两周，每月" /></label></div><div class="config-foot"><label class="near-days">临近更新天数<input v-model.number="configDraft.nearDays" type="number" min="1" /></label><button class="primary">保存配置</button></div></form></div><section class="danger-panel"><div><h3>清理调试数据</h3><p>清空全部项目、进展历史、审计记录和导入批次。用户与系统字典不会受影响。</p></div><button class="danger-button" @click="showClearProjects = true">清空项目数据</button></section></div></section>
    <div v-if="importRows.length" class="modal-backdrop"><section class="modal import-modal"><button class="close-button" @click="importRows = []; importResult = null">×</button><div class="section-kicker">数据导入</div><h2>导入项目表格</h2><p>已识别 {{ importRows.length }} 行，确认后才会写入台账。</p><div class="import-summary"><b>新增 {{ importResult?.created || 0 }}</b><b>更新 {{ importResult?.updated || 0 }}</b><b>错误 {{ importResult?.errors?.length || 0 }}</b></div><div v-if="importResult?.errors?.length" class="import-errors"><p v-for="item in importResult.errors" :key="item.row">第 {{ item.row }} 行：{{ item.message }}</p></div><div class="import-preview"><table><thead><tr><th>省份</th><th>项目名称</th><th>年份</th><th>具体类型</th><th>负责人</th><th>阶段</th><th>当前进展</th></tr></thead><tbody><tr v-for="p in importRows.slice(0, 15)" :key="`${p.province}-${p.name}-${p.projectYear}`"><td>{{ p.province }}</td><td>{{ p.name }}</td><td>{{ p.projectYear }}</td><td>{{ p.specificType || '未识别' }}</td><td>{{ p.expansionOwner || '未识别' }}</td><td>{{ p.stage }}</td><td>{{ p.progress || '未识别' }}</td></tr></tbody></table></div><div class="modal-actions"><button @click="importRows = []; importResult = null">取消</button><button v-if="!importResult?.done" class="primary" @click="confirmImport">确认导入</button><button v-else class="primary" @click="importRows = []; importResult = null">完成</button></div></section></div>
  </div>
    <div v-if="showUserForm" class="modal-backdrop"><form class="modal small-modal" @submit.prevent="createUser"><button type="button" class="close-button" @click="showUserForm = false">×</button><div class="section-kicker">用户管理</div><h2>新增用户</h2><div class="form-grid"><label>姓名<input v-model="newUser.name" required /></label><label>账号<input v-model="newUser.username" required /></label><label>初始密码<input v-model="newUser.password" type="password" minlength="8" required /></label><label>角色<select v-model="newUser.role"><option value="user">普通用户</option><option value="admin">管理员</option></select></label></div><div class="modal-actions"><button type="button" @click="showUserForm = false">取消</button><button class="primary">创建用户</button></div></form></div>
    <div v-if="showPasswordForm" class="modal-backdrop"><form class="modal small-modal" @submit.prevent="changePassword"><button type="button" class="close-button" @click="showPasswordForm = false">×</button><div class="section-kicker">账号安全</div><h2>修改我的密码</h2><p class="muted">修改后下次登录请使用新密码。</p><label>当前密码<input v-model="passwordForm.currentPassword" type="password" autocomplete="current-password" required /></label><label>新密码<input v-model="passwordForm.newPassword" type="password" minlength="8" autocomplete="new-password" required /></label><label>确认新密码<input v-model="passwordForm.confirmPassword" type="password" minlength="8" autocomplete="new-password" required /></label><div class="modal-actions"><button type="button" @click="showPasswordForm = false">取消</button><button class="primary">保存密码</button></div></form></div>
    <div v-if="showClearProjects" class="modal-backdrop"><section class="modal small-modal danger-modal"><button type="button" class="close-button" @click="showClearProjects = false">×</button><div class="danger-icon">!</div><div class="section-kicker danger-kicker">不可逆操作</div><h2>确认清空项目数据？</h2><p class="muted">这会永久删除全部项目、进展历史、审计记录和导入批次，且无法恢复。用户账号和系统字典会保留。</p><div class="modal-actions"><button type="button" @click="showClearProjects = false">取消</button><button type="button" class="danger-button" @click="clearProjects">确认清空</button></div></section></div>
    <div v-if="deleteTarget" class="modal-backdrop"><section class="modal small-modal danger-modal"><button type="button" class="close-button" :disabled="deletingProject" @click="deleteTarget = null">×</button><div class="danger-icon">!</div><div class="section-kicker danger-kicker">不可逆操作</div><h2>确认删除项目？</h2><p class="muted">将永久删除“{{ deleteTarget.name }}”及其进展历史和审计记录，且无法恢复。</p><div class="modal-actions"><button type="button" :disabled="deletingProject" @click="deleteTarget = null">取消</button><button type="button" class="danger-button" :disabled="deletingProject" @click="deleteProject">{{ deletingProject ? '删除中...' : '确认删除' }}</button></div></section></div>
    <div v-if="correctionTarget" class="modal-backdrop"><form class="modal" @submit.prevent="saveCorrection"><button type="button" class="close-button" @click="correctionTarget = null">×</button><div class="section-kicker">历史修正</div><h2>修正历史记录</h2><p class="muted">原始记录不会被覆盖，修正内容将作为新的追加记录保存。</p><div class="form-grid"><label class="wide-field">进展<textarea v-model="correction.progress" required /></label><label class="wide-field">问题说明<textarea v-model="correction.problemDescription" /></label><label class="wide-field">下一步计划<textarea v-model="correction.nextPlan" /></label><label class="wide-field">修正说明<textarea v-model="correction.correctionNote" required /></label></div><div class="modal-actions"><button type="button" @click="correctionTarget = null">取消</button><button class="primary">保存修正</button></div></form></div>
    <div v-if="showForm && formMode !== 'progress'" class="direct-editor-backdrop"><form class="direct-editor" @submit.prevent="saveForm"><button type="button" class="close-button" @click="showForm = false">×</button><div class="section-kicker">项目基本信息</div><h2>{{ formMode === 'new' ? '新增项目' : '编辑基本信息' }}</h2><div class="direct-editor-scroll"><div class="form-grid"><label>省份<select v-model="form.province"><option v-for="x in dict.provinces" :key="x">{{ x }}</option></select></label><label>项目年份<input v-model="form.projectYear" required /></label><label class="wide-field">项目名称<input v-model="form.name" required /></label><label>来源<input v-model="form.source" placeholder="选填" /></label><label>项目类型<select v-model="form.type"><option value="">未选择</option><option v-for="x in dict.projectTypes" :key="x">{{ x }}</option></select></label><label>具体类型<input v-model="form.specificType" /></label><label>主责部门<input v-model="form.responsibleDepartment" /></label><label>拓展组项目负责人<select v-model="form.expansionOwner"><option value="">未选择</option><option v-for="owner in ownerOptions" :key="owner">{{ owner }}</option></select></label><label class="wide-field">项目牵头参与单位<input v-model="form.leadParticipatingUnits" /></label><label>项目总金额（万元）<input v-model.number="form.totalAmount" type="number" step="0.01" /></label><label>智芯合同金额（万元）<input v-model.number="form.contractAmount" type="number" step="0.01" /></label><label>申报成功时间<input v-model="form.successDate" type="date" /></label><label>智芯角色<input v-model="form.zhixinRole" /></label><label>智芯内部支撑部门<input v-model="form.internalSupportDepartment" /></label><label>省公司支撑部门<input v-model="form.provincialSupportDepartment" /></label><label class="wide-field">政府主管单位或其他相关单位<input v-model="form.governmentUnit" /></label><template v-if="formMode === 'new'"><label>项目阶段<select v-model="form.stage"><option v-for="x in dict.stages" :key="x">{{ x }}</option></select></label><label>健康状态<select v-model="form.health"><option v-for="x in dict.healthStatuses" :key="x">{{ x }}</option></select></label><label class="wide-field">当前进展<textarea v-model="form.progress" /></label><label class="wide-field">问题说明<textarea v-model="form.problemDescription" /></label><div class="wide-field tag-field"><span>问题标签</span><label v-for="x in dict.problemTags" :key="x" class="check-label"><input type="checkbox" :value="x" v-model="form.problemTags" />{{ x }}</label></div><label class="wide-field">下一步计划<textarea v-model="form.nextPlan" /></label><label>更新周期<select v-model="form.updateCycle"><option v-for="x in dict.updateCycles" :key="x">{{ x }}</option></select></label><label v-if="form.updateCycle === '自定义'">周期天数<input v-model.number="form.customCycleDays" type="number" min="1" /></label><label class="wide-field">本次备注<textarea v-model="note" /></label></template></div></div><div class="modal-actions"><button type="button" @click="showForm = false">取消</button><button class="primary">保存</button></div></form></div>
    <div v-if="showExportColumns" class="modal-backdrop"><form class="modal small-modal" @submit.prevent="exportLedger"><button type="button" class="close-button" @click="showExportColumns = false">×</button><div class="section-kicker">数据导出</div><h2>选择导出列</h2><p class="muted">请选择需要写入 Excel 的字段。</p><div class="export-columns"><label v-for="([key, title]) in exportColumns" :key="key" class="check-label"><input type="checkbox" :checked="exportColumnKeys.includes(key)" @change="toggleExportColumn(key)" />{{ title }}</label></div><div class="modal-actions"><button type="button" @click="showExportColumns = false">取消</button><button type="submit" class="primary">导出</button></div></form></div>
    <section v-if="view === 'mine'" class="mine-overlay"><div class="mine-sheet"><div class="section-kicker">账号中心</div><h1>我的</h1><section class="mine-profile"><span class="mine-avatar">{{ user.name.slice(0, 1) }}</span><div><strong>{{ user.name }}</strong><small>{{ user.role === 'admin' ? '管理员' : '普通用户' }} · {{ user.username }}</small></div></section><button class="mine-action" @click="openPasswordForm">修改密码 <span>›</span></button><button v-if="user.role === 'admin'" class="mine-action" @click="changeView('config')">系统配置 <span>›</span></button><button class="mine-logout" @click="logout">退出登录</button></div></section>
    <nav class="mobile-bottom-nav"><button :class="{ active: view === 'ledger' }" @click="changeView('ledger')">▣<span>台账</span></button><button :class="{ active: view === 'panorama' }" @click="changeView('panorama')">⌖<span>全景</span></button><button :class="{ active: view === 'mine' || view === 'config' }" @click="changeView('mine')">◎<span>我的</span></button></nav>
</template>
