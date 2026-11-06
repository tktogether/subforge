<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { IconPlus, IconRefresh, IconPencil, IconTrash } from '@tabler/icons-vue'
import { useCrud, type CrudResource } from '../../composables/useCrud'
import { notify } from '../../composables/useNotify'
import { confirmAction } from '../../composables/useConfirm'
import { fieldHelp } from '../../content/help'
import SfPageHead from './SfPageHead.vue'
import SfDialog from './SfDialog.vue'
import SfField from './SfField.vue'
import SfSwitch from './SfSwitch.vue'
import SfEmpty from './SfEmpty.vue'
import SfInfoTip from './SfInfoTip.vue'
import { IconInbox, IconAlertTriangle } from '@tabler/icons-vue'

type Row = Record<string, unknown>

export interface ResourceField {
  key: string
  label: string
  type: string
  hint?: string
  example?: string
  options?: { title: string; value: number | string }[]
  default?: boolean | number | string
}

export interface ResourceColumn {
  key: string
  title: string
  mono?: boolean
  width?: string
  align?: 'left' | 'right' | 'center'
}

const props = withDefaults(
  defineProps<{
    pageTitle: string
    pageDesc?: string
    columns: ResourceColumn[]
    fields: ResourceField[]
    resource: CrudResource<Row>
    readonlyMode?: boolean
    emptyTitle?: string
    emptyDesc?: string
    addLabel?: string
  }>(),
  { pageDesc: '', readonlyMode: false, emptyTitle: '暂无数据', emptyDesc: '', addLabel: '新增' },
)

const { items, error, loading, load, save, destroy } = useCrud<Row>(props.resource)

const dialog = ref(false)
const draft = ref<Row>({})
const editing = ref<Row | null>(null)
const page = ref(1)
const itemsPerPage = ref(10)
const loadError = ref('')

const allColumns = computed<ResourceColumn[]>(() => {
  if (props.readonlyMode) {
    return props.columns
  }
  return props.columns.concat([{ key: 'actions', title: '操作', width: '96px', align: 'right' }])
})

const pageCount = computed(() => Math.max(1, Math.ceil(items.value.length / itemsPerPage.value)))
const pagedItems = computed(() => {
  const start = (page.value - 1) * itemsPerPage.value
  return items.value.slice(start, start + itemsPerPage.value)
})

watch([items, itemsPerPage], () => {
  page.value = Math.min(page.value, pageCount.value)
})

function setPageSize(next: number): void {
  itemsPerPage.value = next
  page.value = 1
}

function fieldHint(field: ResourceField): string {
  if (field.hint !== undefined && field.hint !== '') {
    return field.hint
  }
  const fromHelp = fieldHelp(field.key)
  return fromHelp !== undefined && fromHelp.hint !== undefined ? fromHelp.hint : ''
}

function fieldExample(field: ResourceField): string {
  if (field.example !== undefined && field.example !== '') {
    return field.example
  }
  const fromHelp = fieldHelp(field.key)
  return fromHelp !== undefined && fromHelp.example !== undefined ? fromHelp.example : ''
}

function blank(): Row {
  const row: Row = {}
  for (const field of props.fields) {
    if (field.default !== undefined) {
      row[field.key] = field.default
      continue
    }
    row[field.key] = field.type === 'switch' ? true : field.type === 'number' ? 0
      : field.type === 'select' ? (field.options !== undefined && field.options.length > 0 ? field.options[0].value : '')
      : ''
  }
  return row
}

function reload(): void {
  void load().then(() => {
    loadError.value = error.value
  })
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

async function askDelete(row: Row): Promise<void> {
  const name = typeof row['name'] === 'string' ? String(row['name']) : ''
  const ok = await confirmAction({
    title: '确认删除？',
    body: (name !== '' ? '「' + name + '」' : '该条目') + '删除后不可恢复。',
    confirmText: '删除',
    danger: true,
  })
  if (!ok) {
    return
  }
  const done = await destroy(row)
  notify(done ? '已删除' : error.value, done ? 'ok' : 'error')
}

reload()
</script>

<template>
  <div>
    <SfPageHead :title="props.pageTitle" :desc="props.pageDesc">
      <template #actions>
        <button v-if="!props.readonlyMode" class="sf-btn sf-btn--primary" type="button" data-testid="add" @click="openCreate">
          <IconPlus :size="15" :stroke-width="2" />
          {{ props.addLabel }}
        </button>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="refresh" :disabled="loading" @click="reload">
          <IconRefresh :size="15" :stroke-width="1.8" :class="{ 'sf-res-spin': loading }" />
          刷新
        </button>
      </template>
    </SfPageHead>

    <div v-if="loadError !== ''" class="sf-note sf-note--danger sf-rise">
      <IconAlertTriangle :size="16" :stroke-width="1.8" class="sf-note__icon" />
      <span class="sf-note__body">{{ loadError }}</span>
      <button class="sf-btn sf-btn--ghost sf-btn--sm" type="button" @click="reload">重试</button>
    </div>

    <div class="sf-rise sf-d1">
      <div class="res-bar">
        <span class="sf-pagination__info">共 {{ items.length }} 条</span>
        <span class="sf-spacer" />
        <label class="res-size">
          <span class="sf-faint" style="font-size: 12px">每页</span>
          <span class="sf-select-wrap" style="width: 76px">
            <select class="sf-select sf-select--sm" data-testid="page-size" :value="String(itemsPerPage)" @change="setPageSize(Number(($event.target as HTMLSelectElement).value))">
              <option v-for="n in [10, 25, 50, 100]" :key="n" :value="String(n)">{{ n }}</option>
            </select>
          </span>
        </label>
      </div>

      <div class="sf-table-wrap">
        <table class="sf-table" data-testid="table">
          <thead>
            <tr>
              <th v-for="col in allColumns" :key="col.key" :style="{ width: col.width, textAlign: col.align }">{{ col.title }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && items.length === 0">
              <td v-for="col in allColumns" :key="col.key"><span class="sf-skeleton" style="width: 60%" /></td>
            </tr>
            <tr v-else-if="items.length === 0">
              <td :colspan="allColumns.length" style="padding: 0">
                <SfEmpty :title="props.emptyTitle" :desc="props.emptyDesc">
                  <template #icon><IconInbox :size="30" :stroke-width="1.4" /></template>
                  <template v-if="!props.readonlyMode" #action>
                    <button class="sf-btn sf-btn--primary sf-btn--sm" type="button" @click="openCreate">
                      <IconPlus :size="14" :stroke-width="2" />
                      {{ props.addLabel }}
                    </button>
                  </template>
                </SfEmpty>
              </td>
            </tr>
            <tr v-for="row in pagedItems" v-else :key="String(row['id'] ?? JSON.stringify(row).slice(0, 40))">
              <td v-for="col in allColumns" :key="col.key" :class="{ 'sf-table__num': col.mono }" :style="{ textAlign: col.align }">
                <slot :name="'cell-' + col.key" :row="row" :value="row[col.key]">
                  <template v-if="col.key === 'enabled'">
                    <span class="sf-status">
                      <span class="sf-led" :class="row['enabled'] ? 'sf-led--ok' : 'sf-led--idle'" />
                      <span>{{ row['enabled'] ? '启用' : '停用' }}</span>
                    </span>
                  </template>
                  <template v-else-if="col.key === 'actions'">
                    <span class="sf-table__actions">
                      <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm" type="button" aria-label="编辑" data-testid="edit" @click="openEdit(row)">
                        <IconPencil :size="15" :stroke-width="1.7" />
                      </button>
                      <button class="sf-btn sf-btn--subtle sf-btn--icon sf-btn--sm sf-btn--del" type="button" aria-label="删除" data-testid="delete" @click="askDelete(row)">
                        <IconTrash :size="15" :stroke-width="1.7" />
                      </button>
                    </span>
                  </template>
                  <template v-else>{{ row[col.key] }}</template>
                </slot>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="items.length > itemsPerPage" class="sf-pagination" style="border: none; padding: 12px 2px">
        <span class="sf-pagination__info">PAGE {{ page }} / {{ pageCount }}</span>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--ghost sf-btn--icon sf-btn--sm" type="button" aria-label="上一页" :disabled="page <= 1" @click="page = page - 1">‹</button>
        <button class="sf-btn sf-btn--ghost sf-btn--icon sf-btn--sm" type="button" aria-label="下一页" :disabled="page >= pageCount" @click="page = page + 1">›</button>
      </div>
    </div>

    <SfDialog :open="dialog" :label="editing === null ? 'CREATE' : 'EDIT'" :title="editing === null ? '新增' : '编辑'" @close="dialog = false">
      <form class="sf-form" @submit.prevent="submit">
        <SfField v-for="field in props.fields" :key="field.key" :hint="fieldHint(field)">
          <template #label>
            <span class="sf-field__label">{{ field.label }} <SfInfoTip v-if="fieldHint(field) !== ''" :text="fieldHint(field)" /></span>
          </template>
          <SfSwitch
            v-if="field.type === 'switch'"
            v-model="draft[field.key] as boolean"
            data-testid="field-switch"
          />
          <input
            v-else-if="field.type === 'number'"
            v-model.number="draft[field.key]"
            type="number"
            class="sf-input sf-table__num"
            data-testid="field-number"
          />
          <span v-else-if="field.type === 'select'" class="sf-select-wrap">
            <select v-model="draft[field.key]" class="sf-select" data-testid="field-select">
              <option v-for="opt in field.options ?? []" :key="String(opt.value)" :value="opt.value">{{ opt.title }}</option>
            </select>
          </span>
          <input
            v-else
            v-model="draft[field.key] as string"
            class="sf-input"
            :placeholder="fieldExample(field)"
            data-testid="field-text"
            spellcheck="false"
          />
        </SfField>
      </form>
      <template #foot>
        <button class="sf-btn sf-btn--ghost" type="button" data-testid="dialog-cancel" @click="dialog = false">取消</button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="dialog-submit" @click="submit">保存</button>
      </template>
    </SfDialog>
  </div>
</template>

<style scoped>
.res-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}

.res-size {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.sf-select--sm {
  height: 28px;
  font-size: 12.5px;
  padding: 0 26px 0 8px;
}

.sf-select--sm + span {
  display: none;
}

.sf-select-wrap::after {
  right: 10px;
  margin-top: -2px;
}

.sf-res-spin {
  animation: res-rotate 0.9s linear infinite;
}

@keyframes res-rotate {
  to {
    transform: rotate(360deg);
  }
}
</style>
