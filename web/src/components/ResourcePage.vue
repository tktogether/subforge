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
    row[field.key] = field.type === 'switch' ? true : field.type === 'number' ? 0 : ''
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
    <v-data-table :items="items" :headers="headers" density="comfortable" hover data-testid="table">
      <template #item.enabled="{ value }">
        <v-chip :color="value ? 'success' : 'default'" size="small" variant="tonal">
          {{ value ? '启用' : '停用' }}
        </v-chip>
      </template>
      <template #item.subscription_url="{ value }">
        <span class="d-inline-block text-truncate" style="max-width: 380px">{{ value }}</span>
      </template>
      <template #item.link="{ value }">
        <span class="d-inline-block text-truncate" style="max-width: 380px">{{ value }}</span>
      </template>
      <template #item.token="{ value }">
        <code>{{ value }}</code>
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
