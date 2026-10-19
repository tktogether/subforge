<script setup lang="ts">
import { computed, ref } from 'vue'
import { useCrud, type CrudResource } from '../composables/useCrud'
import { notify } from '../composables/useNotify'
import PageHeader from './PageHeader.vue'
import AsyncSection from './AsyncSection.vue'
import { fieldHelp, type EmptyStateHelp } from '../content/help'

type Row = Record<string, any>

export interface ResourceColumn {
  key: string
  title: string
}

export interface ResourceField {
  key: string
  label: string
  type: string
  hint?: string
  example?: string
  // T10: select 字段的下拉选项（title 显示、value 承载，值类型 number/string）。
  // 不传 options 的字段行为与改动前完全一致。
  options?: { title: string; value: number | string }[]
}

const props = defineProps<{
  title: string
  subtitle: string
  columns: ResourceColumn[]
  fields: ResourceField[]
  resource: CrudResource<Row>
  readonlyMode?: boolean
  emptyText?: string
  empty?: EmptyStateHelp
  helpKey?: string
  rankKey?: string
}>()

const { items, error, loading, load, save, destroy } = useCrud<Row>(props.resource)

const dialog = ref(false)
const confirmOpen = ref(false)
const draft = ref<Row>({})
const editing = ref<Row | null>(null)
const pendingDelete = ref<Row | null>(null)

const headers = computed(() => {
  if (props.readonlyMode === true) {
    return props.columns
  }
  return props.columns.concat([{ key: 'actions', title: '操作' }])
})

// 排名：按 rankKey 升序稳定排序，值 -> 1-based 序号；值取不到时兜底显示原值。
const rankMap = computed(() => {
  const map = new Map<unknown, number>()
  if (!props.rankKey) {
    return map
  }
  const key = props.rankKey
  const sorted = [...items.value].sort((a, b) => {
    const av = (a as Row)[key]
    const bv = (b as Row)[key]
    if (av === bv) return 0
    if (av === undefined || av === null) return 1
    if (bv === undefined || bv === null) return -1
    if (typeof av === 'number' && typeof bv === 'number') return av - bv
    return String(av).localeCompare(String(bv))
  })
  sorted.forEach((row, index) => {
    const value = (row as Row)[key]
    if (!map.has(value)) {
      map.set(value, index + 1)
    }
  })
  return map
})

function rankOf(item: Row): unknown {
  if (!props.rankKey) {
    return (item as Row)[props.rankKey as string]
  }
  const value = item[props.rankKey]
  return rankMap.value.get(value) ?? value
}

// W6B 表格：给操作列单元格挂类（vuetify-overrides.css 据此做「单行不折 + 紧凑按钮」）。
// 不能写进 headers 数组——ResourcePage.spec.ts 用 toEqual 断言 actions 表头对象只有 key/title，
// 所以改走 v-data-table 的表级 cell-props。
function cellPropsFor(data: { column?: { key?: string | null } }): Record<string, string> {
  return data.column?.key === 'actions' ? { class: 'sf-table__actions' } : {}
}

const emptyText = computed(() => props.emptyText ?? '暂无数据')

function fieldHint(field: ResourceField): string {
  const explicit = field.hint
  if (explicit !== undefined && explicit !== '') {
    return explicit
  }
  const fromHelp = fieldHelp(field.key)
  return fromHelp !== undefined && fromHelp.hint !== undefined ? fromHelp.hint : ''
}

function fieldExample(field: ResourceField): string {
  const explicit = field.example
  if (explicit !== undefined && explicit !== '') {
    return explicit
  }
  const fromHelp = fieldHelp(field.key)
  return fromHelp !== undefined && fromHelp.example !== undefined ? fromHelp.example : ''
}

function blank(): Row {
  const row: Row = {}
  for (const field of props.fields) {
    // T10: select 默认取第一个选项的 value（无 options 时退回 ''）。
    // 不兜这一层时 select 会落成 ''，而 rule_config 在后端是 int64（internal/store/store.go:190、
    // internal/api/subscription.go:275-281），新增表单不改下拉就会提交 "" → 400。
    row[field.key] = field.type === 'switch' ? true : field.type === 'number' ? 0
      : field.type === 'select' ? (field.options && field.options.length > 0 ? field.options[0].value : '')
      : ''
  }
  return row
}

function reload(): void {
  void load()
}

function openCreate(): void {
  draft.value = blank()
  editing.value = null
  dialog.value = true
}

function openEdit(row: Row): void {
  draft.value = { ...row }
  editing.value = row
  dialog.value = true
}

async function submit(): Promise<void> {
  const isCreate = editing.value === null
  const ok = await save(draft.value, editing.value)
  if (ok) {
    dialog.value = false
    notify(isCreate ? '已新增' : '已更新')
  } else {
    notify(error.value, 'error')
  }
}

function askDelete(row: Row): void {
  pendingDelete.value = row
  confirmOpen.value = true
}

async function doDelete(): Promise<void> {
  const row = pendingDelete.value
  confirmOpen.value = false
  if (row === null) {
    return
  }
  const ok = await destroy(row)
  notify(ok ? '已删除' : error.value, ok ? 'ok' : 'error')
}

reload()
</script>

<template>
  <PageHeader :title="props.title" :subtitle="props.subtitle" :help-key="props.helpKey">
    <template #actions>
      <v-btn
        v-if="props.readonlyMode !== true"
        color="primary"
        prepend-icon="mdi-plus"
        data-testid="add"
        @click="openCreate"
      >
        新增
      </v-btn>
      <v-btn variant="text" prepend-icon="mdi-refresh" data-testid="refresh" @click="reload">刷新</v-btn>
    </template>
  </PageHeader>

  <AsyncSection
    :loading="loading"
    :error="error"
    :empty="props.empty !== undefined && items.length === 0 ? props.empty : (items.length === 0)"
    :empty-text="emptyText"
    @retry="reload"
  >
    <v-data-table :items="items" :headers="headers" :cell-props="cellPropsFor" density="comfortable" hover data-testid="table" class="resource-table">
      <template #item.enabled="{ value }">
        <span class="sf-status">
          <span class="sf-status__dot" :class="value ? 'sf-status__dot--ok' : 'sf-status__dot--muted'"></span>
          <span>{{ value ? '启用' : '停用' }}</span>
        </span>
      </template>
      <template #item.subscription_url="{ value }">
        <span class="resource-mono resource-truncate" style="max-width: 380px" :title="value">{{ value }}</span>
      </template>
      <template #item.link="{ value }">
        <span class="resource-mono resource-truncate" style="max-width: 380px" :title="value">{{ value }}</span>
      </template>
      <template #item.token="{ value }">
        <code class="resource-mono resource-truncate" :title="value">{{ value }}</code>
      </template>
      <template v-if="rankKey" #[`item.${rankKey}`]="{ item }">
        {{ rankOf(item) }}
      </template>
      <template #item.actions="{ item }">
        <v-btn
          icon="mdi-pencil"
          size="small"
          variant="text"
          data-testid="edit"
          @click="openEdit(item)"
        />
        <v-btn
          icon="mdi-delete"
          size="small"
          variant="text"
          color="error"
          data-testid="delete"
          @click="askDelete(item)"
        />
      </template>
    </v-data-table>
  </AsyncSection>

  <v-dialog v-model="dialog" max-width="560">
    <v-card>
      <v-card-title>{{ editing === null ? '新增' : '编辑' }}</v-card-title>
      <v-card-text>
        <template v-for="field in props.fields" :key="field.key">
          <v-switch
            v-if="field.type === 'switch'"
            v-model="draft[field.key]"
            :label="field.label"
            color="primary"
            hide-details
            data-testid="field-switch"
          />
          <v-text-field
            v-else-if="field.type === 'number'"
            v-model.number="draft[field.key]"
            :label="field.label"
            type="number"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="field-number"
          />
          <v-select
            v-else-if="field.type === 'select'"
            v-model="draft[field.key]"
            :items="field.options ?? []"
            item-title="title"
            item-value="value"
            :label="field.label"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            hide-details
            data-testid="field-select"
          />
          <v-text-field
            v-else
            v-model="draft[field.key]"
            :label="field.label"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="field-text"
          />
          <div v-if="fieldHint(field) !== ''" data-testid="field-hint" class="text-caption text-medium-emphasis mb-2">
            {{ fieldHint(field) }}
          </div>
          <div v-if="fieldExample(field) !== ''" data-testid="field-example" class="text-caption text-medium-emphasis mb-2">
            示例：{{ fieldExample(field) }}
          </div>
        </template>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="dialog-cancel" @click="dialog = false">取消</v-btn>
        <v-btn color="primary" data-testid="dialog-submit" @click="submit">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog v-model="confirmOpen" max-width="420">
    <v-card>
      <v-card-title>确认删除</v-card-title>
      <v-card-text>删除后不可恢复，确定继续？</v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" data-testid="confirm-cancel" @click="confirmOpen = false">取消</v-btn>
        <v-btn color="error" data-testid="confirm-delete" @click="doDelete">删除</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>


<style scoped>
.resource-table {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
}
.resource-table :deep(thead th) {
  background: var(--sf-surface-raised);
  color: var(--sf-text-muted);
  font-size: 12px;
  font-weight: 600;
  position: sticky;
  top: 0;
}
.resource-mono {
  font-family: var(--sf-font-mono);
  font-variant-numeric: tabular-nums;
}
.resource-truncate {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.resource-table :deep(.text-end),
.resource-table :deep(.text-right) {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
</style>
