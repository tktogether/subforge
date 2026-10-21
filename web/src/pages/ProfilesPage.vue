<script setup lang="ts">
import { ref } from 'vue'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'
import { endpoints, type Profile } from '../api/endpoints'
import { notify } from '../composables/useNotify'
import { errorMessage } from '../composables/useAsync'

// 模板列的显示上限：超过就截断，完整内容放在 title 里。
const TEMPLATE_MAX = 60

const EMPTY_PROFILE: Profile = {
  id: 0,
  name: '',
  description: '',
  config: '',
  pipeline: '',
  enabled: true,
  sort_order: 0,
  rule_count: 0,
  group_count: 0,
}

const profiles = ref<Profile[]>([])
const loading = ref(false)
const error = ref('')

// editingId === 0 表示「新建」，否则为被编辑方案的 id。
const editingId = ref(0)
const dialog = ref(false)
const draft = ref({
  name: '',
  description: '',
  config: '',
  pipeline: '',
  enabled: true,
  sort_order: 0,
})

const confirmOpen = ref(false)
const deleteTarget = ref<Profile>(EMPTY_PROFILE)

const headers = [
  { key: 'name', title: '名称' },
  { key: 'description', title: '说明' },
  { key: 'config', title: '模板' },
  { key: 'pipeline', title: '管线' },
  { key: 'rule_count', title: '规则' },
  { key: 'group_count', title: '分组' },
  { key: 'enabled', title: '启用' },
  { key: 'sort_order', title: '顺序' },
  { key: 'actions', title: '操作' },
]

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    profiles.value = await endpoints.listProfiles()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

// truncateTemplate 把过长的模板文本截断到 60 字。
function truncateTemplate(text: string): string {
  if (text.length <= TEMPLATE_MAX) {
    return text
  }
  return text.slice(0, TEMPLATE_MAX) + '…'
}

function templateText(config: string): string {
  return config === '' ? '默认' : truncateTemplate(config)
}

function pipelineText(pipeline: string): string {
  return pipeline === '' ? '默认' : pipeline
}

function openCreate(): void {
  editingId.value = 0
  draft.value = {
    name: '',
    description: '',
    config: '',
    pipeline: '',
    enabled: true,
    sort_order: 0,
  }
  dialog.value = true
}

function openEdit(row: Profile): void {
  editingId.value = row.id
  draft.value = {
    name: row.name,
    description: row.description,
    config: row.config,
    pipeline: row.pipeline,
    enabled: row.enabled,
    sort_order: row.sort_order,
  }
  dialog.value = true
}

async function save(): Promise<void> {
  const body = {
    name: draft.value.name,
    description: draft.value.description,
    config: draft.value.config,
    pipeline: draft.value.pipeline,
    enabled: draft.value.enabled,
    sort_order: draft.value.sort_order,
  }
  try {
    if (editingId.value === 0) {
      await endpoints.createProfile(body)
    } else {
      await endpoints.updateProfile(editingId.value, body)
    }
    notify('已保存')
    dialog.value = false
    await load()
  } catch (err) {
    // 后端的冲突文案（例如「方案名称 “X” 已存在」）原样透出。
    notify(errorMessage(err), 'error')
  }
}

function askDelete(row: Profile): void {
  deleteTarget.value = row
  confirmOpen.value = true
}

function cancelDelete(): void {
  confirmOpen.value = false
}

async function confirmDelete(): Promise<void> {
  const target = deleteTarget.value
  confirmOpen.value = false
  try {
    await endpoints.deleteProfile(target.id)
    notify('已删除')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

load()
</script>

<template>
  <div class="profiles-page">
    <PageHeader title="订阅方案" subtitle="一个方案 = 一套模板 + 规则 + 分组；令牌绑定方案后彼此隔离">
      <template #actions>
        <v-btn color="primary" prepend-icon="mdi-plus" data-testid="profile-new" @click="openCreate">
          新增
        </v-btn>
        <v-btn variant="text" prepend-icon="mdi-refresh" data-testid="profile-refresh" @click="load">
          刷新
        </v-btn>
      </template>
    </PageHeader>

    <v-alert type="info" variant="tonal" class="mb-4" data-testid="profiles-help">
      <div class="text-body-2">
        <p class="mb-1"><strong>方案是什么：</strong>一个订阅方案 = 一套模板（INI config）+ 一套规则 + 一套分组；令牌绑定方案后，订阅只输出该方案的规则与分组。</p>
        <p class="mb-1"><strong>未分配：</strong>profile_id = 0 的规则与分组就是旧全局池，没绑定方案的令牌行为与改动前完全一致。</p>
        <p class="mb-1"><strong>删除方案：</strong>删除方案前要先把规则/分组移走或删除，否则后端会拒绝（方案下还有内容）。</p>
        <p class="mb-0"><strong>模板留空：</strong>模板留空 = 用默认配置；管线留空 = 用全局回落。</p>
      </div>
    </v-alert>

    <AsyncSection
      :loading="loading"
      :error="error"
      :empty="profiles.length === 0"
      empty-text="还没有订阅方案"
      @retry="load"
    >
      <v-data-table
        :items="profiles"
        :headers="headers"
        density="comfortable"
        hover
        data-testid="profile-table"
        class="profiles-table"
      >
        <template #item.name="{ item }">
          <span data-testid="profile-row">{{ item.name }}</span>
        </template>
        <template #item.config="{ item }">
          <span class="profiles-truncate" :title="item.config">{{ templateText(item.config) }}</span>
        </template>
        <template #item.pipeline="{ item }">
          <span>{{ pipelineText(item.pipeline) }}</span>
        </template>
        <template #item.enabled="{ item }">
          <span class="sf-status">
            <span class="sf-status__dot" :class="item.enabled ? 'sf-status__dot--ok' : 'sf-status__dot--muted'"></span>
            <span>{{ item.enabled ? '启用' : '停用' }}</span>
          </span>
        </template>
        <template #item.actions="{ item }">
          <v-btn
            icon="mdi-pencil"
            size="small"
            variant="text"
            data-testid="profile-edit"
            @click="openEdit(item)"
          />
          <v-btn
            icon="mdi-delete"
            size="small"
            variant="text"
            color="error"
            data-testid="profile-delete"
            @click="askDelete(item)"
          />
        </template>
      </v-data-table>
    </AsyncSection>

    <v-dialog v-model="dialog" max-width="560">
      <v-card>
        <v-card-title>{{ editingId === 0 ? '新建方案' : '编辑方案' }}</v-card-title>
        <v-card-text>
          <v-text-field
            v-model="draft.name"
            label="名称"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-name"
          />
          <v-text-field
            v-model="draft.description"
            label="说明"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-description"
          />
          <v-text-field
            v-model="draft.config"
            label="模板"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-config"
          />
          <v-text-field
            v-model="draft.pipeline"
            label="管线"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-pipeline"
          />
          <v-switch
            v-model="draft.enabled"
            label="启用"
            color="primary"
            hide-details
            class="mb-2"
            data-testid="profile-enabled"
          />
          <v-text-field
            v-model.number="draft.sort_order"
            label="顺序"
            type="number"
            variant="outlined"
            density="comfortable"
            class="mb-2"
            data-testid="profile-sort"
          />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" data-testid="profile-cancel" @click="dialog = false">取消</v-btn>
          <v-btn color="primary" data-testid="profile-save" @click="save">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="confirmOpen" max-width="420">
      <v-card>
        <v-card-title>确认删除</v-card-title>
        <v-card-text>删除方案「{{ deleteTarget.name }}」后不可恢复，确定继续？</v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" data-testid="profile-delete-cancel" @click="cancelDelete">取消</v-btn>
          <v-btn color="error" data-testid="profile-delete-confirm" @click="confirmDelete">删除</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
.profiles-table {
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
}
.profiles-truncate {
  display: inline-block;
  max-width: 380px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
