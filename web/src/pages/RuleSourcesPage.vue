<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type RuleSource, type RuleSourceUpdateResult, type RuleSourcesResponse } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { pageHelp, glossaryFor } from '../content/help'
import PageHeader from '../components/PageHeader.vue'
import AsyncSection from '../components/AsyncSection.vue'
import { formatBytes } from '../format'

const help = pageHelp('rule-sources')

const loading = ref(false)
const error = ref('')
const data = ref<RuleSourcesResponse | null>(null)
const sources = ref<RuleSource[]>([])
const manifestPath = ref('')
const defaultManifestPath = ref('')
const outputDir = ref('')
const exists = ref(false)
const hint = ref('')
const manifestTemplate = ref('')

const initDialog = ref(false)
const initForce = ref(false)

const rollbackDialog = ref(false)
const rollbackName = ref('')

const mrsDialog = ref(false)
const mrsName = ref('')
const mrsBehavior = ref('domain')

const updateResults = ref<RuleSourceUpdateResult[]>([])
const showUpdateResults = ref(false)

const emptyState = computed(() => help.emptyState)

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const r = await endpoints.listRuleSources()
    data.value = r
    manifestPath.value = r.manifest_path
    defaultManifestPath.value = r.default_manifest_path ?? ''
    outputDir.value = r.output_dir ?? ''
    exists.value = r.exists ?? false
    hint.value = r.hint ?? ''
    manifestTemplate.value = r.manifest_template ?? ''
    sources.value = r.sources ?? []
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function openInit(): void {
  if (exists.value) {
    initForce.value = true
    initDialog.value = true
  } else {
    void doInit(false)
  }
}

async function doInit(force: boolean): Promise<void> {
  try {
    const r = await endpoints.initRuleManifest(force)
    notify('已生成: ' + r.path + (r.created ? '（新建）' : '（覆盖）'), 'ok')
    initDialog.value = false
    initForce.value = false
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

async function confirmInit(): Promise<void> {
  await doInit(true)
}

function cancelInit(): void {
  initDialog.value = false
  initForce.value = false
}

async function updateAll(): Promise<void> {
  try {
    const r = await endpoints.updateRuleSources()
    updateResults.value = r.results ?? []
    showUpdateResults.value = true
    notify('更新完成', 'ok')
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

function openRollback(name: string): void {
  rollbackName.value = name
  rollbackDialog.value = true
}

async function doRollback(): Promise<void> {
  try {
    const r = await endpoints.rollbackRuleSource(rollbackName.value)
    notify('已回滚: ' + r.path, 'ok')
    rollbackDialog.value = false
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

function cancelRollback(): void {
  rollbackDialog.value = false
  rollbackName.value = ''
}

function openMrs(name: string): void {
  mrsName.value = name
  mrsBehavior.value = 'domain'
  mrsDialog.value = true
}

async function doMrs(): Promise<void> {
  try {
    const r = await endpoints.generateMRS(mrsName.value, mrsBehavior.value)
    notify('已生成: ' + r.path + ' (' + formatBytes(r.bytes) + ')', 'ok')
    mrsDialog.value = false
    await load()
  } catch (err) {
    notify(errorMessage(err), 'error')
  }
}

function cancelMrs(): void {
  mrsDialog.value = false
}

onMounted(() => {
  void load()
})
</script>

<template>
  <PageHeader title="规则源" :subtitle="help.oneLiner" help-key="rule-sources">
    <template #actions>
      <v-btn data-testid="init-manifest" :color="exists ? undefined : 'primary'" variant="tonal" @click="openInit">
        生成默认清单
      </v-btn>
      <v-btn data-testid="update-all" variant="tonal" @click="updateAll">全部更新</v-btn>
    </template>
  </PageHeader>

      <!-- Manifest card -->
    <v-card data-testid="manifest-card" class="mb-4">
      <v-card-title>清单路径</v-card-title>
      <v-card-text>
        <div data-testid="manifest-path">生效路径：{{ manifestPath }}</div>
        <div v-if="defaultManifestPath !== ''" data-testid="manifest-default-path">默认路径：{{ defaultManifestPath }}</div>
        <div v-if="outputDir !== ''" data-testid="manifest-output-dir">输出目录：{{ outputDir }}</div>
        <div v-if="!exists" data-testid="manifest-hint" class="text-warning mt-2">{{ hint }}</div>
        <v-chip v-if="exists" data-testid="manifest-exists" color="success" size="small" class="mt-2">已就绪</v-chip>
      </v-card-text>
    </v-card>

<AsyncSection :loading="loading" :error="error" :empty="sources.length === 0 ? (emptyState ?? false) : false" empty-text="暂无规则源" @retry="load">
    <!-- Source table -->
    <v-table data-testid="source-table">
      <thead>
        <tr>
          <th>名称</th>
          <th>URL</th>
          <th>格式</th>
          <th>输出目录</th>
          <th>版本数</th>
          <th>最新版本</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="s in sources" :key="s.name" data-testid="source-row">
          <td>{{ s.name }}</td>
          <td>{{ s.url }}</td>
          <td>{{ s.format }}</td>
          <td>{{ s.dir }}</td>
          <td>{{ s.versions.length }}</td>
          <td>
            <span v-if="s.latest === ''" data-testid="no-version">暂无版本</span>
            <span v-else>{{ s.latest }}</span>
          </td>
          <td>
            <v-btn size="small" variant="text" data-testid="source-update" @click="updateAll">更新</v-btn>
            <v-btn size="small" variant="text" data-testid="source-rollback" :disabled="s.versions.length === 0" @click="openRollback(s.name)">回滚</v-btn>
            <v-btn size="small" variant="text" color="primary" data-testid="source-mrs" :disabled="s.versions.length === 0" @click="openMrs(s.name)">生成 MRS</v-btn>
          </td>
        </tr>
      </tbody>
    </v-table>

    <!-- Version list per source -->
    <v-expansion-panels class="mt-4">
      <v-expansion-panel v-for="s in sources" :key="'versions-' + s.name" :title="s.name + ' 的版本'">
        <v-expansion-panel-text>
          <div v-for="v in s.versions" :key="v" data-testid="version-item">{{ v }}</div>

        </v-expansion-panel-text>
      </v-expansion-panel>
    </v-expansion-panels>

    <!-- MRS format + retention explanation -->
    <v-card class="mt-4" variant="tonal">
      <v-card-title>MRS 格式说明</v-card-title>
      <v-card-text>{{ glossaryFor('MRS') }}</v-card-text>
      <v-card-text class="text-caption text-medium-emphasis">版本保留策略：每个源保留最近 5 个版本，可回滚；更新失败不会删掉旧版本。</v-card-text>
    </v-card>

    <!-- Update results -->
    <div v-if="showUpdateResults" data-testid="update-results" class="mt-4">
      <v-card>
        <v-card-title>更新结果</v-card-title>
        <v-card-text>
          <div v-for="(r, i) in updateResults" :key="i">
            <div>来源：{{ r.source }}</div>
            <div>版本：{{ r.version }}</div>
            <div>路径：{{ r.path }}</div>
            <span v-if="r.from_cache" data-testid="from-cache">命中缓存</span>
            <div v-if="r.err !== ''" data-testid="source-update-error" class="text-error">{{ r.err }}</div>
            <div v-if="r.err !== ''" class="text-caption text-medium-emphasis">该源保留上一版本，产物不受影响</div>
            <div v-if="r.err !== ''" class="text-caption text-medium-emphasis">该源保留上一版本，产物不受影响</div>
          </div>
        </v-card-text>
      </v-card>
    </div>
  </AsyncSection>

  <!-- Init confirm dialog -->
  <v-dialog v-model="initDialog" max-width="480">
    <v-card>
      <v-card-title>覆盖现有清单？</v-card-title>
      <v-card-text>
        会覆盖现有 manifest.yaml，旧文件备份为 {{ manifestPath }}.bak
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="cancelInit">取消</v-btn>
        <v-btn color="primary" @click="confirmInit">确认覆盖</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Rollback confirm dialog -->
  <v-dialog v-model="rollbackDialog" max-width="480">
    <v-card>
      <v-card-title>确认回滚 {{ rollbackName }}</v-card-title>
      <v-card-text>
        回滚会把该规则源的最新版本换成上一个版本，订阅产物在下一次生成时才会变化。回滚只切换 latest 指针到指定版本文件，不改配置。
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="cancelRollback">取消</v-btn>
        <v-btn color="primary" @click="doRollback">确认回滚</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- MRS dialog -->
  <v-dialog v-model="mrsDialog" max-width="480">
    <v-card>
      <v-card-title>生成 MRS - {{ mrsName }}</v-card-title>
      <v-card-text>
        <v-select v-model="mrsBehavior" :items="['domain', 'ipcidr']" label="行为" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="cancelMrs">取消</v-btn>
        <v-btn color="primary" @click="doMrs">生成</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
